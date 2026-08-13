package shopify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStartBulkQueryReturnsOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"bulkOperationRunQuery":{"bulkOperation":{"id":"gid://shopify/BulkOperation/1","status":"CREATED"},"userErrors":[]}}}`))
	}))
	defer server.Close()
	client := NewClientWithEndpoint(server.Client(), func(string, string) string { return server.URL })

	operation, err := client.StartBulkQuery(context.Background(), "test.myshopify.com", "token", "2026-07", "{ products { edges { node { id } } } }")
	if err != nil {
		t.Fatalf("StartBulkQuery() error = %v", err)
	}
	if operation.ID != "gid://shopify/BulkOperation/1" || operation.Status != BulkStatusCreated {
		t.Fatalf("operation = %#v", operation)
	}
}

func TestStartBulkQueryReturnsUserError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"bulkOperationRunQuery":{"bulkOperation":null,"userErrors":[{"field":["query"],"message":"Invalid query"}]}}}`))
	}))
	defer server.Close()
	client := NewClientWithEndpoint(server.Client(), func(string, string) string { return server.URL })

	_, err := client.StartBulkQuery(context.Background(), "test.myshopify.com", "token", "2026-07", "invalid")
	if err == nil || !strings.Contains(err.Error(), "Invalid query") {
		t.Fatalf("error = %v, want user error", err)
	}
}

func TestGetBulkOperationReturnsDownloadURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"bulkOperation":{"id":"gid://shopify/BulkOperation/1","status":"COMPLETED","objectCount":"12","fileSize":"100","url":"https://storage.example/result.jsonl","partialDataUrl":null}}}`))
	}))
	defer server.Close()
	client := NewClientWithEndpoint(server.Client(), func(string, string) string { return server.URL })

	operation, err := client.GetBulkOperation(context.Background(), "test.myshopify.com", "token", "2026-07", "gid://shopify/BulkOperation/1")
	if err != nil {
		t.Fatalf("GetBulkOperation() error = %v", err)
	}
	if operation.Status != BulkStatusCompleted || operation.URL != "https://storage.example/result.jsonl" || operation.ObjectCount != "12" {
		t.Fatalf("operation = %#v", operation)
	}
}

func TestDownloadJSONLReturnsStreamingBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"id\":1}\n{\"id\":2}\n"))
	}))
	defer server.Close()
	client := NewClientWithEndpoint(server.Client(), func(string, string) string { return server.URL })

	body, err := client.DownloadJSONL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("DownloadJSONL() error = %v", err)
	}
	defer body.Close()
	content, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "{\"id\":1}\n{\"id\":2}\n" {
		t.Fatalf("content = %q", content)
	}
}
