package adapter

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"order-service/config"
	"order-service/internal/core/domain/entity"
	"order-service/internal/core/service"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type middlewareAdapter struct {
	cfg        *config.Config
	jwtService service.JwtServiceInterface
	redis      *redis.Client
}

type MiddlewareAdapterInterface interface {
	CheckToken() fiber.Handler
	DistanceCheck() fiber.Handler
}

func NewMiddlewareAdapter(cfg *config.Config, jwtService service.JwtServiceInterface, redis *redis.Client) MiddlewareAdapterInterface {
	return &middlewareAdapter{
		cfg:        cfg,
		jwtService: jwtService,
		redis:      redis,
	}
}

func (m *middlewareAdapter) HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // radius bumi dalam kilometer

	dLat := (lat2 - lat1) * (math.Pi / 180)
	dLon := (lon2 - lon1) * (math.Pi / 180)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180))*math.Cos(lat2*(math.Pi/180))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c //kilometer
}

func (m *middlewareAdapter) DistanceCheck() fiber.Handler {
	return func(c fiber.Ctx) error {
		latParam := c.Query("lat")
		lngParam := c.Query("lng")

		if latParam == "" || lngParam == "" {
			log.Error().
				Str("lat", latParam).
				Str("lng", lngParam).
				Str("source", "internal.adapter.middlewareAdapter.DistanceCheck").
				Msg("missing or invalid lat or lng")

			return fiber.NewError(fiber.StatusBadRequest, "missing or invalid lat or lng")
		}

		lat, err := strconv.ParseFloat(latParam, 64)
		if err != nil {
			log.Error().
				Err(err).
				Str("lat", latParam).
				Str("source", "internal.adapter.middlewareAdapter.DistanceCheck").
				Msg("failed parse latitude")

			return fiber.NewError(fiber.StatusBadRequest, "missing or invalid lat or lng")
		}

		lng, err := strconv.ParseFloat(lngParam, 64)
		if err != nil {
			log.Error().
				Err(err).
				Str("lng", lngParam).
				Str("source", "internal.adapter.middlewareAdapter.DistanceCheck").
				Msg("failed parse longitude")

			return fiber.NewError(fiber.StatusBadRequest, "missing or invalid lat or lng")
		}

		latRef, err := strconv.ParseFloat(m.cfg.App.LatitudeRef, 64)
		if err != nil {
			log.Error().
				Err(err).
				Str("latitude_ref", m.cfg.App.LatitudeRef).
				Str("source", "internal.adapter.middlewareAdapter.DistanceCheck").
				Msg("failed parse latitude reference")

			return fiber.NewError(fiber.StatusInternalServerError, "internal server error")
		}

		lngRef, err := strconv.ParseFloat(m.cfg.App.LongitudeRef, 64)
		if err != nil {
			log.Error().
				Err(err).
				Str("longitude_ref", m.cfg.App.LongitudeRef).
				Str("source", "internal.adapter.middlewareAdapter.DistanceCheck").
				Msg("failed parse longitude reference")

			return fiber.NewError(fiber.StatusInternalServerError, "internal server error")
		}

		distance := m.HaversineDistance(latRef, lngRef, lat, lng)

		if distance > float64(m.cfg.App.MaxDistance) {
			log.Warn().
				Float64("distance_km", distance).
				Float64("max_distance_km", float64(m.cfg.App.MaxDistance)).
				Float64("latitude", lat).
				Float64("longitude", lng).
				Str("source", "internal.adapter.middlewareAdapter.DistanceCheck").
				Msg("distance too far")

			return fiber.NewError(fiber.StatusBadRequest, "distance too far")
		}

		return c.Next()
	}
}

func (m *middlewareAdapter) CheckToken() fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing or invalid token")
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		_, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			log.Error().
				Err(err).
				Str("source", "internal.adapter.middlewareAdapter.CheckToken").
				Msg("failed validate token")

			return fiber.NewError(fiber.StatusUnauthorized, "invalid token")
		}

		getSession, err := m.redis.Get(c.Context(), tokenString).Result()
		if err != nil || len(getSession) == 0 {
			log.Error().
				Err(err).
				Str("source", "internal.adapter.middlewareAdapter.CheckToken").
				Msg("session not found")

			return fiber.NewError(fiber.StatusUnauthorized, "session not found")
		}

		var jwtUserData entity.JwtUserData

		if err := json.Unmarshal([]byte(getSession), &jwtUserData); err != nil {
			log.Error().
				Err(err).
				Str("source", "internal.adapter.middlewareAdapter.CheckToken").
				Msg("failed unmarshal jwt user data")

			return fiber.NewError(fiber.StatusInternalServerError, "failed parse session")
		}

		path := c.Path()
		segments := strings.Split(strings.Trim(path, "/"), "/")

		// membatasi akses user dengan role customer supaya tidak bisa mengakses endpoint yang diawali dengan /admin
		if jwtUserData.RoleName == "Customer" &&
			len(segments) > 0 &&
			segments[0] == "admin" {

			log.Error().
				Str("user_role", jwtUserData.RoleName).
				Str("path", path).
				Str("source", "internal.adapter.middlewareAdapter.CheckToken").
				Msg("customer cannot access admin routes")

			return fiber.NewError(fiber.StatusForbidden, "customer cannot access admin routes")
		}

		c.Locals("user", getSession)

		return c.Next()
	}
}
