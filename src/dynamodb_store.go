package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const customerHistoryIndexName = "customer_id-created_at-index"

type DynamoDBStore struct {
	client      *dynamodb.Client
	tableName   string
	strongReads bool
}

func NewDynamoDBStore(ctx context.Context, cfg Config) (*DynamoDBStore, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return nil, err
	}

	return &DynamoDBStore{
		client:      dynamodb.NewFromConfig(awsCfg),
		tableName:   cfg.DynamoDBTableName,
		strongReads: cfg.DynamoDBStrong,
	}, nil
}

func (s *DynamoDBStore) CreateCart(ctx context.Context, cart Cart) (Cart, error) {
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:                cartToDynamoItem(cart),
		ConditionExpression: aws.String("attribute_not_exists(cart_id)"),
	})
	if err != nil {
		return Cart{}, err
	}
	return cart, nil
}

func (s *DynamoDBStore) GetCart(ctx context.Context, cartID int64) (Cart, error) {
	output, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: strconv.FormatInt(cartID, 10)},
		},
		ConsistentRead: aws.Bool(s.strongReads),
	})
	if err != nil {
		return Cart{}, err
	}
	if len(output.Item) == 0 {
		return Cart{}, ErrCartNotFound
	}
	return cartFromDynamoItem(output.Item)
}

func (s *DynamoDBStore) UpsertItem(ctx context.Context, cartID int64, item CartItem) error {
	cart, err := s.getCartStronglyConsistent(ctx, cartID)
	if err != nil {
		return err
	}

	updated := false
	for i := range cart.Items {
		if cart.Items[i].ProductID == item.ProductID {
			cart.Items[i].Quantity = item.Quantity
			updated = true
			break
		}
	}
	if !updated {
		cart.Items = append(cart.Items, item)
	}
	sort.Slice(cart.Items, func(i, j int) bool {
		return cart.Items[i].ProductID < cart.Items[j].ProductID
	})
	cart.UpdatedAt = time.Now().UTC()

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:                cartToDynamoItem(cart),
		ConditionExpression: aws.String("attribute_exists(cart_id)"),
	})
	if err != nil {
		var conditional *types.ConditionalCheckFailedException
		if errors.As(err, &conditional) {
			return ErrCartNotFound
		}
		return err
	}
	return nil
}

func (s *DynamoDBStore) getCartStronglyConsistent(ctx context.Context, cartID int64) (Cart, error) {
	output, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: strconv.FormatInt(cartID, 10)},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return Cart{}, err
	}
	if len(output.Item) == 0 {
		return Cart{}, ErrCartNotFound
	}
	return cartFromDynamoItem(output.Item)
}

func (s *DynamoDBStore) ListCartsByCustomer(ctx context.Context, customerID int64) ([]Cart, error) {
	output, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String(customerHistoryIndexName),
		KeyConditionExpression: aws.String("customer_id = :customer_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":customer_id": &types.AttributeValueMemberN{Value: strconv.FormatInt(customerID, 10)},
		},
		ScanIndexForward: aws.Bool(false),
	})
	if err != nil {
		return nil, err
	}

	carts := make([]Cart, 0, len(output.Items))
	for _, item := range output.Items {
		cart, err := cartFromDynamoItem(item)
		if err != nil {
			return nil, err
		}
		carts = append(carts, cart)
	}
	return carts, nil
}

func cartToDynamoItem(cart Cart) map[string]types.AttributeValue {
	items := make(map[string]types.AttributeValue, len(cart.Items))
	for _, item := range cart.Items {
		items[strconv.FormatInt(item.ProductID, 10)] = &types.AttributeValueMemberN{
			Value: strconv.FormatInt(item.Quantity, 10),
		}
	}

	return map[string]types.AttributeValue{
		"cart_id":     &types.AttributeValueMemberS{Value: strconv.FormatInt(cart.ShoppingCartID, 10)},
		"customer_id": &types.AttributeValueMemberN{Value: strconv.FormatInt(cart.CustomerID, 10)},
		"created_at":  &types.AttributeValueMemberS{Value: cart.CreatedAt.UTC().Format(time.RFC3339Nano)},
		"updated_at":  &types.AttributeValueMemberS{Value: cart.UpdatedAt.UTC().Format(time.RFC3339Nano)},
		"items":       &types.AttributeValueMemberM{Value: items},
	}
}

func cartFromDynamoItem(item map[string]types.AttributeValue) (Cart, error) {
	cartID, err := getString(item, "cart_id")
	if err != nil {
		return Cart{}, err
	}
	cartIDValue, err := strconv.ParseInt(cartID, 10, 64)
	if err != nil {
		return Cart{}, err
	}

	customerID, err := getNumber(item, "customer_id")
	if err != nil {
		return Cart{}, err
	}
	customerIDValue, err := strconv.ParseInt(customerID, 10, 64)
	if err != nil {
		return Cart{}, err
	}

	createdAtRaw, err := getString(item, "created_at")
	if err != nil {
		return Cart{}, err
	}
	updatedAtRaw, err := getString(item, "updated_at")
	if err != nil {
		return Cart{}, err
	}

	createdAt, err := time.Parse(time.RFC3339Nano, createdAtRaw)
	if err != nil {
		return Cart{}, err
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtRaw)
	if err != nil {
		return Cart{}, err
	}

	itemsAttr, ok := item["items"].(*types.AttributeValueMemberM)
	if !ok {
		return Cart{}, fmt.Errorf("items attribute missing or invalid")
	}

	items := make([]CartItem, 0, len(itemsAttr.Value))
	for productIDRaw, qtyAttr := range itemsAttr.Value {
		productID, err := strconv.ParseInt(productIDRaw, 10, 64)
		if err != nil {
			return Cart{}, err
		}
		quantityRaw, ok := qtyAttr.(*types.AttributeValueMemberN)
		if !ok {
			return Cart{}, fmt.Errorf("invalid quantity for product %s", productIDRaw)
		}
		quantity, err := strconv.ParseInt(quantityRaw.Value, 10, 64)
		if err != nil {
			return Cart{}, err
		}
		items = append(items, CartItem{ProductID: productID, Quantity: quantity})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductID < items[j].ProductID
	})

	return Cart{
		ShoppingCartID: cartIDValue,
		CustomerID:     customerIDValue,
		Items:          items,
		CreatedAt:      createdAt.UTC(),
		UpdatedAt:      updatedAt.UTC(),
	}, nil
}

func getString(item map[string]types.AttributeValue, key string) (string, error) {
	value, ok := item[key]
	if !ok {
		return "", fmt.Errorf("missing attribute %s", key)
	}
	member, ok := value.(*types.AttributeValueMemberS)
	if !ok {
		return "", fmt.Errorf("attribute %s is not a string", key)
	}
	return member.Value, nil
}

func getNumber(item map[string]types.AttributeValue, key string) (string, error) {
	value, ok := item[key]
	if !ok {
		return "", fmt.Errorf("missing attribute %s", key)
	}
	member, ok := value.(*types.AttributeValueMemberN)
	if !ok {
		return "", fmt.Errorf("attribute %s is not numeric", key)
	}
	return member.Value, nil
}
