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

func (v VestigoMCP) SearchDocuments(ctx context.Context, req *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	searchApi := "/api/v1/search/simple"
	if input.Model != "BM25" {
		searchApi = "/api/v1/search/ann/" + input.Model
	}
	ref, err := url.Parse(searchApi)
	if err != nil {
		return nil, SearchOutput{}, err
	}
	searchUrl := v.endpoint.ResolveReference(ref)
	q := searchUrl.Query()
	q.Set("q", input.Query)
	q.Set("n", strconv.Itoa(input.Count))
	searchUrl.RawQuery = q.Encode()
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
