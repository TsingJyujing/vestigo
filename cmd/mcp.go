package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/tsingjyujing/vestigo/controller"
)

type SearchInput struct {
	Model string `json:"model" jsonschema:"the name of the model to search"`
	Query string `json:"query" jsonschema:"the query to search for"`
	Count int    `json:"n" jsonschema:"the number of results to return"`
}

type SearchOutput struct {
	Results []controller.SearchResultItem `json:"results" jsonschema:"the search results"`
}

type VestigoMCP struct {
	client   *http.Client
	endpoint url.URL
}

func (v VestigoMCP) GetUrl(relativePath string, parameters map[string]string) (*url.URL, error) {
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
	searchApi := "/api/v1/search/simple"
	if input.Model != "BM25" {
		searchApi = "/api/v1/search/ann/" + input.Model
	}
	searchUrl, err := v.GetUrl(searchApi, map[string]string{
		"q": input.Query,
		"n": strconv.Itoa(input.Count),
	})
	if err != nil {
		return nil, SearchOutput{}, err
	}
	// Make request
	request := &http.Request{
		Method: http.MethodGet,
		URL:    searchUrl,
	}
	resp, err := v.client.Do(request)
	if err != nil {
		return nil, SearchOutput{}, err
	}
	defer resp.Body.Close()
	// Parse response
	var result []controller.SearchResultItem
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, SearchOutput{}, err
	}
	return nil, SearchOutput{Results: result}, nil
}

type ListModelInput struct {
	// No input parameters
}

type ListModelOutput struct {
	Models []string `json:"models" jsonschema:"the list of available embedding models"`
}

func (v VestigoMCP) ListModels(ctx context.Context, req *mcp.CallToolRequest, input ListModelInput) (*mcp.CallToolResult, ListModelOutput, error) {
	listModelUrl, err := v.GetUrl("/api/v1/search/models", nil)
	if err != nil {
		return nil, ListModelOutput{}, err
	}
	// Make request
	request := &http.Request{
		Method: http.MethodGet,
		URL:    listModelUrl,
	}
	resp, err := v.client.Do(request)
	if err != nil {
		return nil, ListModelOutput{}, err
	}
	defer resp.Body.Close()
	// Parse response
	var result []string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, ListModelOutput{}, err
	}
	result = append(result, "BM25") // Add BM25 model
	return nil, ListModelOutput{Models: result}, nil
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
			mcp.AddTool(server, &mcp.Tool{Name: "search_documents", Description: "Search documents with query"}, v.SearchDocuments)
			mcp.AddTool(server, &mcp.Tool{Name: "list_models", Description: "List available models, BM25 is most basic & fastest one, if we can not find anything, we can use other embedding based ANN search models"}, v.ListModels)
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
