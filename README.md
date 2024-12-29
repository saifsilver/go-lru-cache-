# Go Multi-Storage LRU Cache with TTL and Automated Expiry

A generic **Least Recently Used (LRU) Cache** implementation in Go, with support for multiple storage backends:

- **MemoryStorage**: In-memory storage for fast access.
- **FileStorage**: Local file-based persistence for cache data.
- **RedisStorage**: Distributed caching with Redis.
- **S3Storage**: Cloud-based caching with AWS S3.

## Features

- **Capacity-based eviction**: Automatically removes the least recently used items when the cache exceeds its capacity.
- **Time-to-Live (TTL)**: Items expire and are removed after a specified duration.
- **Multi-storage backend support**: Choose from in-memory, file-based, Redis, or S3 storage.
- **Automated expiry checking**: Background processes ensure expired items are cleaned up.
- **Thread-safe operations**: Proper synchronization for all storage types.

---

## Installation

Clone the repository and include the necessary dependencies:
```bash
go get github.com/go-redis/redis/v8
go get github.com/aws/aws-sdk-go
```

---

## Example Usage

### Memory Storage
```go
memoryStorage := NewMemoryStorage()

memoryStorage.Put("testKey", "testValue", 5*time.Second)
value, err := memoryStorage.Get("testKey")
if err != nil {
	log.Println(err)
} else {
	log.Println("Memory Storage: ", value)
}
```

### File Storage
```go
fileStorage := NewFileStorage("cache.json")

fileStorage.Put("fileKey", "fileValue", 10*time.Second)
value, err := fileStorage.Get("fileKey")
if err != nil {
	log.Println(err)
} else {
	log.Println("File Storage: ", value)
}
```

### Redis Storage
```go
redisStorage := NewRedisStorage("localhost:6379", "", 0)

redisStorage.Put("redisKey", "redisValue", 10*time.Second)
value, err := redisStorage.Get("redisKey")
if err != nil {
	log.Println(err)
} else {
	log.Println("Redis Storage: ", value)
}
```

### S3 Storage
```go
s3Storage := NewS3Storage("my-bucket", "cache.json", "us-east-1")

s3Storage.Put("s3Key", "s3Value", 15*time.Second)
value, err := s3Storage.Get("s3Key")
if err != nil {
	log.Println(err)
} else {
	log.Println("S3 Storage: ", value)
}
```

---

## Contributing

Contributions are welcome! Feel free to submit a pull request or open an issue.

---

## License

This project is licensed under the MIT License.

---
