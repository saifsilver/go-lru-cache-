package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/net/context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type CacheStorage interface {
	Get(key string) (interface{}, error)
	Put(key string, value interface{}, ttl time.Duration) error
	Delete(key string) error
	Stop() error
}

type MemoryStorage struct {
	cache map[string]cacheItem
	mu    sync.Mutex
}

type cacheItem struct {
	Value  interface{}
	Expiry time.Time
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{cache: make(map[string]cacheItem)}
}

func (m *MemoryStorage) Get(key string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.cache[key]
	if !exists || time.Now().After(item.Expiry) {
		delete(m.cache, key)
		return nil, fmt.Errorf("key not found or expired")
	}
	return item.Value, nil
}

func (m *MemoryStorage) Put(key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache[key] = cacheItem{Value: value, Expiry: time.Now().Add(ttl)}
	return nil
}

func (m *MemoryStorage) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.cache, key)
	return nil
}

func (m *MemoryStorage) Stop() error {
	return nil
}

// FileStorage

type FileStorage struct {
	filePath string
	cache    map[string]cacheItem
	mu       sync.Mutex
}

func NewFileStorage(filePath string) *FileStorage {
	fs := &FileStorage{filePath: filePath, cache: make(map[string]cacheItem)}
	fs.loadFromFile()
	return fs
}

func (f *FileStorage) loadFromFile() {
	data, err := ioutil.ReadFile(f.filePath)
	if err == nil {
		_ = json.Unmarshal(data, &f.cache)
	}
}

func (f *FileStorage) saveToFile() {
	data, _ := json.Marshal(f.cache)
	_ = ioutil.WriteFile(f.filePath, data, 0644)
}

func (f *FileStorage) Get(key string) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	item, exists := f.cache[key]
	if !exists || time.Now().After(item.Expiry) {
		delete(f.cache, key)
		f.saveToFile()
		return nil, fmt.Errorf("key not found or expired")
	}
	return item.Value, nil
}

func (f *FileStorage) Put(key string, value interface{}, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cache[key] = cacheItem{Value: value, Expiry: time.Now().Add(ttl)}
	f.saveToFile()
	return nil
}

func (f *FileStorage) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.cache, key)
	f.saveToFile()
	return nil
}

func (f *FileStorage) Stop() error {
	f.saveToFile()
	return nil
}

// RedisStorage

type RedisStorage struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisStorage(addr, password string, db int) *RedisStorage {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisStorage{client: client, ctx: context.Background()}
}

func (r *RedisStorage) Get(key string) (interface{}, error) {
	val, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("key not found or expired")
	}
	var item cacheItem
	_ = json.Unmarshal([]byte(val), &item)
	if time.Now().After(item.Expiry) {
		r.client.Del(r.ctx, key)
		return nil, fmt.Errorf("key not found or expired")
	}
	return item.Value, nil
}

func (r *RedisStorage) Put(key string, value interface{}, ttl time.Duration) error {
	item := cacheItem{Value: value, Expiry: time.Now().Add(ttl)}
	data, _ := json.Marshal(item)
	return r.client.Set(r.ctx, key, data, ttl).Err()
}

func (r *RedisStorage) Delete(key string) error {
	return r.client.Del(r.ctx, key).Err()
}

func (r *RedisStorage) Stop() error {
	return r.client.Close()
}

// S3Storage

type S3Storage struct {
	s3        *s3.S3
	bucket    string
	cacheFile string
	cache     map[string]cacheItem
	mu        sync.Mutex
}

func NewS3Storage(bucket, cacheFile string, region string) *S3Storage {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))
	s3Client := s3.New(sess)
	storage := &S3Storage{
		s3:        s3Client,
		bucket:    bucket,
		cacheFile: cacheFile,
		cache:     make(map[string]cacheItem),
	}
	storage.loadFromS3()
	return storage
}

func (s *S3Storage) loadFromS3() {
	output, err := s.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.cacheFile),
	})
	if err == nil && output.Body != nil {
		defer output.Body.Close()
		data, _ := ioutil.ReadAll(output.Body)
		_ = json.Unmarshal(data, &s.cache)
	}
}

func (s *S3Storage) saveToS3() {
	data, _ := json.Marshal(s.cache)
	_, _ = s.s3.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.cacheFile),
		Body:   ioutil.NopCloser(stringReader(data)),
	})
}

func stringReader(data []byte) *string {
	str := string(data)
	return &str
}

func (s *S3Storage) Get(key string) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.cache[key]
	if !exists || time.Now().After(item.Expiry) {
		delete(s.cache, key)
		s.saveToS3()
		return nil, fmt.Errorf("key not found or expired")
	}
	return item.Value, nil
}

func (s *S3Storage) Put(key string, value interface{}, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[key] = cacheItem{Value: value, Expiry: time.Now().Add(ttl)}
	s.saveToS3()
	return nil
}

func (s *S3Storage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.cache, key)
	s.saveToS3()
	return nil
}

func (s *S3Storage) Stop() error {
	s.saveToS3()
	return nil
}

// Example Usage
func main() {
	memoryStorage := NewMemoryStorage()
	fileStorage := NewFileStorage("cache.json")
	s3Storage := NewS3Storage("my-bucket", "cache.json", "us-east-1")

	memoryStorage.Put("test", "value", 5*time.Second)
	result, err := memoryStorage.Get("test")
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Memory Storage: ", result)
	}

	fileStorage.Put("fileKey", "fileValue", 10*time.Second)
	fileResult, err := fileStorage.Get("fileKey")
	if err != nil {
		log.Println(err)
	} else {
		log.Println("File Storage: ", fileResult)
	}

	s3Storage.Put("s3Key", "s3Value", 15*time.Second)
	s3Result, err := s3Storage.Get("s3Key")
	if err != nil {
		log.Println(err)
	} else {
		log.Println("S3 Storage: ", s3Result)
	}
}
