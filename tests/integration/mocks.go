package integration

import (
	"context"
	"errors"
	"time"
)

// Mock implementations for testing

// MockTimeoutStore simulates connection timeouts
type MockTimeoutStore struct {
	timeout time.Duration
}

func (m *MockTimeoutStore) GetPathTree(ctx context.Context) (PathTree, error) {
	time.Sleep(m.timeout)
	return PathTree{}, errors.New("connection timeout")
}

func (m *MockTimeoutStore) UpsertPathTree(ctx context.Context, tree PathTree) error {
	time.Sleep(m.timeout)
	return errors.New("connection timeout")
}

func (m *MockTimeoutStore) UpsertChunks(ctx context.Context, chunks []Chunk) error {
	return errors.New("not implemented")
}

func (m *MockTimeoutStore) GetChunksByPage(ctx context.Context, pageSlug string) ([]Chunk, error) {
	return nil, errors.New("not implemented")
}

func (m *MockTimeoutStore) DeleteChunksByPage(ctx context.Context, pageSlug string) error {
	return errors.New("not implemented")
}

func (m *MockTimeoutStore) UpsertLazyPointer(ctx context.Context, pointer LazyPointer) error {
	return errors.New("not implemented")
}

func (m *MockTimeoutStore) GetLazyPointer(ctx context.Context, pageSlug string) (*LazyPointer, error) {
	return nil, errors.New("not implemented")
}

func (m *MockTimeoutStore) SearchText(ctx context.Context, pattern string, filter PathFilter, limit int) ([]Chunk, error) {
	return nil, errors.New("not implemented")
}

func (m *MockTimeoutStore) SearchVector(ctx context.Context, queryVec []float32, filter PathFilter, topK int) ([]SearchHit, error) {
	return nil, errors.New("not implemented")
}

func (m *MockTimeoutStore) SearchHybrid(ctx context.Context, queryVec []float32, pattern string, filter PathFilter, topK int) ([]SearchHit, error) {
	return nil, errors.New("not implemented")
}

// MockTransientFailureStore fails N times then delegates to real store
type MockTransientFailureStore struct {
	failCount    int
	attemptCount int
	realStore    VectorStore
}

func (m *MockTransientFailureStore) GetPathTree(ctx context.Context) (PathTree, error) {
	m.attemptCount++
	if m.attemptCount <= m.failCount {
		return PathTree{}, errors.New("transient network error")
	}
	return m.realStore.GetPathTree(ctx)
}

func (m *MockTransientFailureStore) UpsertPathTree(ctx context.Context, tree PathTree) error {
	return m.realStore.UpsertPathTree(ctx, tree)
}

func (m *MockTransientFailureStore) UpsertChunks(ctx context.Context, chunks []Chunk) error {
	return m.realStore.UpsertChunks(ctx, chunks)
}

func (m *MockTransientFailureStore) GetChunksByPage(ctx context.Context, pageSlug string) ([]Chunk, error) {
	return m.realStore.GetChunksByPage(ctx, pageSlug)
}

func (m *MockTransientFailureStore) DeleteChunksByPage(ctx context.Context, pageSlug string) error {
	return m.realStore.DeleteChunksByPage(ctx, pageSlug)
}

func (m *MockTransientFailureStore) UpsertLazyPointer(ctx context.Context, pointer LazyPointer) error {
	return m.realStore.UpsertLazyPointer(ctx, pointer)
}

func (m *MockTransientFailureStore) GetLazyPointer(ctx context.Context, pageSlug string) (*LazyPointer, error) {
	return m.realStore.GetLazyPointer(ctx, pageSlug)
}

func (m *MockTransientFailureStore) SearchText(ctx context.Context, pattern string, filter PathFilter, limit int) ([]Chunk, error) {
	return m.realStore.SearchText(ctx, pattern, filter, limit)
}

func (m *MockTransientFailureStore) SearchVector(ctx context.Context, queryVec []float32, filter PathFilter, topK int) ([]SearchHit, error) {
	return m.realStore.SearchVector(ctx, queryVec, filter, topK)
}

func (m *MockTransientFailureStore) SearchHybrid(ctx context.Context, queryVec []float32, pattern string, filter PathFilter, topK int) ([]SearchHit, error) {
	return m.realStore.SearchHybrid(ctx, queryVec, pattern, filter, topK)
}

// MockS3Store simulates S3 storage in memory
type MockS3Store struct {
	storage map[string][]byte
}

func (m *MockS3Store) Put(ctx context.Context, key string, content []byte) error {
	m.storage[key] = content
	return nil
}

func (m *MockS3Store) Get(ctx context.Context, key string) ([]byte, error) {
	content, exists := m.storage[key]
	if !exists {
		return nil, errors.New("key not found")
	}
	return content, nil
}

func (m *MockS3Store) Delete(ctx context.Context, key string) error {
	delete(m.storage, key)
	return nil
}

func (m *MockS3Store) GenerateURL(ctx context.Context, key string) (string, error) {
	return "s3://test-bucket/" + key, nil
}

// MockFailingS3Store fails N times
type MockFailingS3Store struct {
	failCount    int
	attemptCount int
}

func (m *MockFailingS3Store) Put(ctx context.Context, key string, content []byte) error {
	return errors.New("not implemented")
}

func (m *MockFailingS3Store) Get(ctx context.Context, key string) ([]byte, error) {
	m.attemptCount++
	if m.attemptCount <= m.failCount {
		return nil, errors.New("S3 connection failed")
	}
	return []byte("content"), nil
}

func (m *MockFailingS3Store) Delete(ctx context.Context, key string) error {
	return errors.New("not implemented")
}

func (m *MockFailingS3Store) GenerateURL(ctx context.Context, key string) (string, error) {
	return "s3://test-bucket/" + key, nil
}

// MockTimeoutS3Store simulates S3 timeouts
type MockTimeoutS3Store struct {
	timeout time.Duration
}

func (m *MockTimeoutS3Store) Put(ctx context.Context, key string, content []byte) error {
	return errors.New("not implemented")
}

func (m *MockTimeoutS3Store) Get(ctx context.Context, key string) ([]byte, error) {
	time.Sleep(m.timeout)
	return nil, errors.New("S3 timeout")
}

func (m *MockTimeoutS3Store) Delete(ctx context.Context, key string) error {
	return errors.New("not implemented")
}

func (m *MockTimeoutS3Store) GenerateURL(ctx context.Context, key string) (string, error) {
	return "s3://test-bucket/" + key, nil
}

// MockEmbedder generates deterministic embeddings for testing
type MockEmbedder struct {
	dimension int
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embedding := make([]float32, m.dimension)
	for i := range embedding {
		// Simple hash-like function for deterministic embeddings
		embedding[i] = float32((len(text)+i)%100) / 100.0
	}
	return embedding, nil
}

func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

func (m *MockEmbedder) Dimension() int {
	return m.dimension
}

// MockTimeoutEmbedder simulates embedding API timeouts
type MockTimeoutEmbedder struct {
	timeout time.Duration
}

func (m *MockTimeoutEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	time.Sleep(m.timeout)
	return nil, errors.New("OpenAI API timeout")
}

func (m *MockTimeoutEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	time.Sleep(m.timeout)
	return nil, errors.New("OpenAI API timeout")
}

func (m *MockTimeoutEmbedder) Dimension() int {
	return 1536
}
