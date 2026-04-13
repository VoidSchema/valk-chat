package cache

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client
var Ctx = context.Background()

func InitRedis() {
	url := os.Getenv("REDIS_URL")
	if url != "" {
		opts, err := redis.ParseURL(url)
		if err != nil {
			log.Fatal("Failed to parse REDIS_URL:", err)
		}
		RDB = redis.NewClient(opts)
	} else {
		host := os.Getenv("REDIS_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("REDIS_PORT")
		if port == "" {
			port = "6379"
		}
		password := os.Getenv("REDIS_PASSWORD")

		RDB = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", host, port),
			Password: password,
			DB:       0,
		})
	}

	_, err := RDB.Ping(Ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	fmt.Println("Redis connection established")
}

// Session management

func SetSession(token string, userID uint, username string, ttl time.Duration) error {
	key := "session:" + token
	err := RDB.HSet(Ctx, key, map[string]interface{}{
		"user_id":  userID,
		"username": username,
	}).Err()
	if err != nil {
		return err
	}
	return RDB.Expire(Ctx, key, ttl).Err()
}

func GetSession(token string) (uint, string, error) {
	key := "session:" + token
	result, err := RDB.HGetAll(Ctx, key).Result()
	if err != nil {
		return 0, "", err
	}
	if len(result) == 0 {
		return 0, "", fmt.Errorf("session not found")
	}

	var userID uint
	fmt.Sscanf(result["user_id"], "%d", &userID)
	return userID, result["username"], nil
}

func DeleteSession(token string) error {
	return RDB.Del(Ctx, "session:"+token).Err()
}

// Rate limiting

func GetDailyCount(key string) (int64, error) {
	today := time.Now().Format("2006-01-02")
	fullKey := fmt.Sprintf("ratelimit:%s:%s", key, today)
	count, err := RDB.Get(Ctx, fullKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

func IncrementDailyCounter(key string) (int64, error) {
	today := time.Now().Format("2006-01-02")
	fullKey := fmt.Sprintf("ratelimit:%s:%s", key, today)
	count, err := RDB.Incr(Ctx, fullKey).Result()
	if err != nil {
		return 0, err
	}
	// Set TTL 48 hours if this is a new key (count == 1)
	if count == 1 {
		RDB.Expire(Ctx, fullKey, 48*time.Hour)
	}
	return count, nil
}
