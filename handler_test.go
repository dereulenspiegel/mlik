package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type clientMock struct {
	mock.Mock
}

func (c *clientMock) Do(r *http.Request) (*http.Response, error) {
	args := c.Called(r)
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestHandlerRedirects(t *testing.T) {
	client := new(clientMock)
	client.On("Do", mock.Anything).Return(&http.Response{}, nil)

	handler := createHandler(client, slog.Default())
	request := httptest.NewRequest(http.MethodGet, "http://example.com/something", nil)
	rec := httptest.NewRecorder()
	handler(rec, request)
	resp := rec.Result()
	assert.Equal(t, http.StatusPermanentRedirect, resp.StatusCode)
	assert.Equal(t, "https://example.com/something", resp.Header.Get("Location"))
	client.AssertNotCalled(t, "Do")
}

func TestHandlerProxies(t *testing.T) {
	client := new(clientMock)
	client.On("Do", mock.Anything).Return(&http.Response{
		StatusCode: http.StatusOK,
	}, nil)

	handler := createHandler(client, slog.Default())
	request := httptest.NewRequest(http.MethodGet, "http://example.com/.well-known/acme-challenge/sometoken", nil)
	rec := httptest.NewRecorder()
	handler(rec, request)
	resp := rec.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Location"))
	client.AssertExpectations(t)
}
