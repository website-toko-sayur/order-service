package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"order-service/internal/core/domain/entity"
	helperSearch "order-service/utils/searchbuilder"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/rs/zerolog/log"
)

type elasticRepository struct {
	opensearchClient *opensearch.Client
}

type ElasticRepositoryInterface interface {
	SearchOrderElastic(ctx context.Context, queryString entity.QueryStringEntity) ([]entity.OrderEntity, int64, int64, error)
	SearchOrderElasticByBuyerId(ctx context.Context, queryString entity.QueryStringEntity, buyerId int64) ([]entity.OrderEntity, int64, int64, error)
}

func NewElasticRepository(opensearchClient *opensearch.Client) ElasticRepositoryInterface {
	return &elasticRepository{opensearchClient: opensearchClient}
}

func (e *elasticRepository) SearchOrderElastic(ctx context.Context, query entity.QueryStringEntity) ([]entity.OrderEntity, int64, int64, error) {
	from := (query.Page - 1) * query.Limit

	mustQueries := []map[string]interface{}{}
	filterQueries := []map[string]interface{}{}

	// fulltext search
	if query.Search != "" {
		mustQueries = append(
			mustQueries,
			helperSearch.MultiMatchQuery(
				query.Search,
				[]string{
					"order_code",
					"status",
					"buyer_name",
				},
			),
		)
	}

	// status filter
	if query.Status != "" {
		filterQueries = append(
			filterQueries,
			helperSearch.TermFilter(
				"status.keyword",
				query.Status,
			),
		)
	}

	searchQuery := map[string]interface{}{
		"from": from,
		"size": query.Limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must":   mustQueries,
				"filter": filterQueries,
			},
		},
		"sort": helperSearch.SortQuery(
			"id",
			"asc",
		),
	}

	body, err := json.Marshal(searchQuery)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.elasticRepository.SearchOrderElastic").
			Msg("failed marshal search query")

		return nil, 0, 0, err
	}

	res, err := e.opensearchClient.Search(
		e.opensearchClient.Search.WithContext(ctx),
		e.opensearchClient.Search.WithIndex("orders"),
		e.opensearchClient.Search.WithBody(bytes.NewReader(body)),
	)

	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.elasticRepository.SearchOrderElastic").
			Msg("failed search orders")

		return nil, 0, 0, err
	}

	defer res.Body.Close()

	if err := helperSearch.ParseOpenSearchError(res); err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.elasticRepository.SearchOrderElastic").
			Msg("opensearch returned error")

		return nil, 0, 0, err
	}

	var result map[string]interface{}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.elasticRepository.SearchOrderElastic").
			Msg("failed decode response")

		return nil, 0, 0, err
	}

	hitsRoot, ok := result["hits"].(map[string]interface{})
	if !ok {
		return nil, 0, 0, errors.New("invalid hits response")
	}

	totalData := 0

	if totalMap, ok := hitsRoot["total"].(map[string]interface{}); ok {
		if value, ok := totalMap["value"].(float64); ok {
			totalData = int(value)
		}
	}

	totalPage := 0

	if query.Limit > 0 {
		totalPage = int(math.Ceil(
			float64(totalData) / float64(query.Limit),
		))
	}

	hits, ok := hitsRoot["hits"].([]interface{})
	if !ok {
		return nil, 0, 0, errors.New("invalid hits data")
	}

	orders := []entity.OrderEntity{}

	for _, hit := range hits {

		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"]
		if !ok {
			continue
		}

		data, err := json.Marshal(source)
		if err != nil {
			continue
		}

		var order entity.OrderEntity

		if err := json.Unmarshal(data, &order); err != nil {
			continue
		}

		orders = append(orders, order)
	}

	return orders, int64(totalData), int64(totalPage), nil
}

func (e *elasticRepository) SearchOrderElasticByBuyerId(ctx context.Context, query entity.QueryStringEntity, buyerId int64) ([]entity.OrderEntity, int64, int64, error) {

	from := (query.Page - 1) * query.Limit

	mustQueries := []map[string]interface{}{}
	filterQueries := []map[string]interface{}{}

	// fulltext search
	if query.Search != "" {
		mustQueries = append(
			mustQueries,
			helperSearch.MultiMatchQuery(
				query.Search,
				[]string{
					"order_code",
					"status",
					"buyer_name",
				},
			),
		)
	}

	// status filter
	if query.Status != "" {
		filterQueries = append(
			filterQueries,
			helperSearch.TermFilter(
				"status.keyword",
				query.Status,
			),
		)
	}

	// buyer id filter
	if buyerId != 0 {
		filterQueries = append(
			filterQueries,
			helperSearch.TermFilter(
				"buyer_id",
				buyerId,
			),
		)
	}

	searchQuery := map[string]interface{}{
		"from": from,
		"size": query.Limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must":   mustQueries,
				"filter": filterQueries,
			},
		},
		"sort": helperSearch.SortQuery(
			"id",
			"desc",
		),
	}

	body, err := json.Marshal(searchQuery)
	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.elasticRepository.SearchOrderElasticByBuyerId").
			Msg("failed marshal search query")

		return nil, 0, 0, err
	}

	res, err := e.opensearchClient.Search(
		e.opensearchClient.Search.WithContext(ctx),
		e.opensearchClient.Search.WithIndex("orders"),
		e.opensearchClient.Search.WithBody(bytes.NewReader(body)),
	)

	if err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.elasticRepository.SearchOrderElasticByBuyerId").
			Msg("failed search orders")

		return nil, 0, 0, err
	}

	defer res.Body.Close()

	if err := helperSearch.ParseOpenSearchError(res); err != nil {

		log.Error().
			Err(err).
			Str("source", "internal.adapter.elasticRepository.SearchOrderElasticByBuyerId").
			Msg("opensearch returned error")

		return nil, 0, 0, err
	}

	var result map[string]interface{}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		log.Error().
			Err(err).
			Str("source", "internal.adapter.elasticRepository.SearchOrderElasticByBuyerId").
			Msg("failed decode response")

		return nil, 0, 0, err
	}

	hitsRoot, ok := result["hits"].(map[string]interface{})
	if !ok {
		return nil, 0, 0, errors.New("invalid hits response")
	}

	totalData := 0

	if totalMap, ok := hitsRoot["total"].(map[string]interface{}); ok {
		if value, ok := totalMap["value"].(float64); ok {
			totalData = int(value)
		}
	}

	totalPage := 0

	if query.Limit > 0 {
		totalPage = int(math.Ceil(
			float64(totalData) / float64(query.Limit),
		))
	}

	hits, ok := hitsRoot["hits"].([]interface{})
	if !ok {
		return nil, 0, 0, errors.New("invalid hits data")
	}

	orders := []entity.OrderEntity{}

	for _, hit := range hits {

		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"]
		if !ok {
			continue
		}

		data, err := json.Marshal(source)
		if err != nil {
			continue
		}

		var order entity.OrderEntity

		if err := json.Unmarshal(data, &order); err != nil {
			continue
		}

		orders = append(orders, order)
	}

	return orders, int64(totalData), int64(totalPage), nil
}
