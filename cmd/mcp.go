package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/tsingjyujing/vestigo/controller"
)

func closeIoReadCloser(rc io.ReadCloser) {
	err := rc.Close()
	if err != nil {
		logger.WithError(err).Error("failed to close response body")
	}
}

type VestigoMCP struct {
	client   *http.Client
	endpoint url.URL
}
type CommonOutput struct {
	Status  string `json:"status" default:"ok" jsonschema:"the status of the response"`
	Message string `json:"message,omitempty" jsonschema:"the message of the response"`
}

// errorResponseHandler checks the HTTP response for errors and returns a CommonOutput and error if any.
func errorResponseHandler(resp http.Response) (CommonOutput, error) {
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		respBody, err := io.ReadAll(resp.Body)
		errorInfo := fmt.Errorf(
			"list models API returned status code %d with content: %s",
			resp.StatusCode,
			respBody,
		)
		if err != nil {
			return CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("list models API returned status code %d and failed to read response body: %v", resp.StatusCode, err),
			}, err
		}
		return CommonOutput{
			Status:  "error",
			Message: errorInfo.Error(),
		}, errorInfo
	}
	return CommonOutput{Status: "ok"}, nil
}

type SearchInput struct {
	Model string `json:"model" jsonschema:"the name of the model for searching"`
	Query string `json:"query" jsonschema:"the query to search for, while using ANN model, it can be a sentence, for BM25 model, use space to separate keywords for AND logic and use OR to separate keywords for OR logic"`
	Count int    `json:"n" jsonschema:"the number of results to return for each model"`
}

type SearchOutput struct {
	CommonOutput
	controller.SearchResponse
}

func (v VestigoMCP) getUrl(relativePath string, parameters map[string]string) (*url.URL, error) {
	u, err := url.Parse(relativePath)
	if err != nil {
		return nil, err
	}
	u = v.endpoint.ResolveReference(u)
	if parameters != nil {
		q := u.Query()
		for k, v := range parameters {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	return u, nil
}

func (v VestigoMCP) SearchDocuments(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	searchUrl, err := v.getUrl(fmt.Sprintf("/api/v1/search/%s", input.Model), map[string]string{
		"q": input.Query,
		"n": strconv.Itoa(input.Count),
	})
	if err != nil {
		return nil, SearchOutput{
			CommonOutput: CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("failed to build search url: %v", err),
			},
		}, err
	}
	// Make request
	resp, err := v.client.Do(&http.Request{
		Method: http.MethodGet,
		URL:    searchUrl,
	})
	if err != nil {
		return nil, SearchOutput{
			CommonOutput: CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("failed to search: %v", err),
			},
		}, err
	}
	defer closeIoReadCloser(resp.Body)
	commonOutput, err := errorResponseHandler(*resp)
	if err != nil {
		return nil, SearchOutput{
			CommonOutput: commonOutput,
		}, err
	}
	// Parse response
	var result controller.SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, SearchOutput{
			CommonOutput: CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("failed to parse search response: %v", err),
			},
		}, err
	}
	return nil, SearchOutput{SearchResponse: result, CommonOutput: commonOutput}, nil
}

type ListModelInput struct {
	// No input parameters
}

type ListModelOutput struct {
	CommonOutput
	Models []string `json:"models" jsonschema:"the list of available embedding models"`
}

func (v VestigoMCP) ListModels(ctx context.Context, req *mcp.CallToolRequest, input ListModelInput) (*mcp.CallToolResult, ListModelOutput, error) {
	listModelUrl, err := v.getUrl("/api/v1/models", nil)
	if err != nil {
		return nil, ListModelOutput{
			CommonOutput: CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("failed to build list models url: %v", err),
			},
		}, err
	}
	// Make request
	resp, err := v.client.Do(&http.Request{
		Method: http.MethodGet,
		URL:    listModelUrl,
	})
	if err != nil {
		return nil, ListModelOutput{
			CommonOutput: CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("failed to list models: %v", err),
			},
		}, err
	}
	defer closeIoReadCloser(resp.Body)
	commonOutput, err := errorResponseHandler(*resp)
	if err != nil {
		return nil, ListModelOutput{
			CommonOutput: commonOutput,
		}, err
	}
	// Parse response
	var result []string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, ListModelOutput{
			CommonOutput: CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("failed to parse list models response: %v", err),
			},
		}, err
	}
	result = append(result, "BM25") // Add BM25 model
	return nil, ListModelOutput{Models: result, CommonOutput: commonOutput}, nil
}

type GetDocumentInput struct {
	DocumentID string `json:"document_id" jsonschema:"the ID of the document to retrieve"`
	// Currently always return text chunks
}

type GetDocumentOutput struct {
	CommonOutput
	Document controller.DocumentWithChunks `json:"document" jsonschema:"the retrieved full document by ID"`
}

func (v VestigoMCP) GetDocument(ctx context.Context, req *mcp.CallToolRequest, input GetDocumentInput) (*mcp.CallToolResult, GetDocumentOutput, error) {
	getDocUrl, err := v.getUrl("/api/v1/doc/"+input.DocumentID, map[string]string{"with_texts": "true"})
	if err != nil {
		return nil, GetDocumentOutput{
			CommonOutput: CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("failed to build get document url: %v", err),
			},
		}, err
	}
	// Make request
	resp, err := v.client.Do(&http.Request{
		Method: http.MethodGet,
		URL:    getDocUrl,
	})
	if err != nil {
		return nil, GetDocumentOutput{
			CommonOutput: CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("failed to get document: %v", err),
			},
		}, err
	}
	defer closeIoReadCloser(resp.Body)
	commonOutput, err := errorResponseHandler(*resp)
	if err != nil {
		return nil, GetDocumentOutput{
			CommonOutput: commonOutput,
		}, err
	}
	// Parse response
	var result controller.DocumentWithChunks
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, GetDocumentOutput{
			CommonOutput: CommonOutput{
				Status:  "error",
				Message: fmt.Sprintf("failed to parse get document response: %v", err),
			},
		}, err
	}
	return nil, GetDocumentOutput{Document: result, CommonOutput: commonOutput}, nil
}

func NewMcpCommand() *cobra.Command {
	var vestigoEndpoint string

	mcpCommand := &cobra.Command{
		Use:   "mcp",
		Short: "Starting MCP server",
		Run: func(cmd *cobra.Command, args []string) {
			parsedURL, err := url.Parse(vestigoEndpoint)
			if err != nil {
				logger.Fatalf("Invalid Vestigo endpoint URL: %v", err)
			}
			v := VestigoMCP{
				client:   http.DefaultClient,
				endpoint: *parsedURL,
			}
			server := mcp.NewServer(&mcp.Implementation{Name: "vestigo-mcp", Title: "MCP server for searching document from Vestigo", Version: "v1.0.0"}, nil)
			mcp.AddTool(server, &mcp.Tool{
				Name: "search_documents",
				Description: "Search text chunks with query and model ID, will return text chunk and document ID, " +
					"for accessing full document, we need to use get_document API",
			}, v.SearchDocuments)
			mcp.AddTool(server, &mcp.Tool{
				Name: "list_models",
				Description: "List available embedding models, BM25 is most basic & fastest one (but not embedding), for " +
					"BM25, we can search by keywords separated by spaces, for other models, we can even ask question directly " +
					"since it will generate sentence embedding",
			}, v.ListModels)
			mcp.AddTool(server, &mcp.Tool{Name: "get_document", Description: "Get full document with all text chunks by ID, with option to include text chunks"}, v.GetDocument)
			if err := server.Run(cmd.Context(), &mcp.StdioTransport{}); err != nil {
				logger.Fatal(err)
			}
		},
	}
	mcpCommand.Flags().StringVarP(
		&vestigoEndpoint,
		"endpoint",
		"e", "http://localhost:8080",
		"Vestigo server endpoint URL",
	)
	return mcpCommand
}
