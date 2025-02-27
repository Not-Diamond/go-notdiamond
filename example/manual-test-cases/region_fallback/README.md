# Region Fallback Test Cases

This directory contains test configurations for the region fallback feature in NotDiamond.

## Overview

Region fallback allows you to specify multiple regions for a model and automatically fall back to alternative regions if the primary region fails. This is useful for:

- Improving reliability by having backup regions
- Handling region-specific outages
- Optimizing for latency by trying closer regions first

## Implementation Details

The region fallback feature has been implemented with the following key components:

1. **Model Naming Convention**: Models can now include a region in their name using the format `provider/model/region` (e.g., `vertex/gemini-pro/us-east4`).

2. **Region Prioritization**:

   - If a region is specified in the request, that specific region is tried first.
   - If no region is specified, the system automatically adds region-specific versions of the model to the front of the models list.

3. **URL Transformation**:

   - For Vertex AI: Updates the host to `{region}-aiplatform.googleapis.com` and the path to include the correct region.
   - For OpenAI: Updates the host to `{region}.api.openai.com` if a region is specified.
   - For Azure: Updates the host to include the region in the format `{endpoint-name}.{region}.api.cognitive.microsoft.com`.

4. **Fallback Mechanism**:
   - If a request to a specific region fails, the system automatically tries the next region in the list.
   - Error tracking and health checks are performed for each region.

## Test Cases

1. **Basic Region Fallback** (`1_region_fallback.go`)

   - Simple ordered fallback for Vertex AI, OpenAI, and Azure
   - Mixed provider fallback

2. **Weighted Region Fallback** (`2_region_fallback_weighted.go`)

   - Weighted distribution across regions
   - Region fallback with timeouts

3. **Error Tracking and Retries** (`3_region_fallback_error_tracking.go`)
   - Region fallback with error tracking
   - Status code-specific retry configuration
   - Backoff between retries

## Region Support by Provider

### Vertex AI

- Supports all Google Cloud regions where Vertex AI is available
- Common regions: `us-central1`, `us-east4`, `us-west1`, `europe-west4`, etc.

### OpenAI

- Limited region support
- Regions: `us` (default), `eu`
- Format: For EU region, the host becomes `api.eu.openai.com`

### Azure OpenAI

- Supports all Azure regions where Azure OpenAI is available
- Common regions: `eastus`, `westus`, `westeurope`, etc.
- Format: `{endpoint-name}.{region}.api.cognitive.microsoft.com`

## Usage

To use these test cases, modify the example application to use one of these configurations:

```go
import (
    "github.com/Not-Diamond/go-notdiamond"
    "github.com/Not-Diamond/go-notdiamond/example/manual-test-cases/region_fallback"
)

func main() {
    // Initialize NotDiamond with region fallback configuration
    client, err := notdiamond.Init(test_region_fallback.RegionFallbackVertexTest)
    if err != nil {
        panic(err)
    }

    // Use the client...
}
```

## Testing

The region fallback functionality has been tested with:

1. **Unit Tests**: Testing the ordering logic and URL transformation.
2. **Integration Tests**: Testing the actual fallback behavior with real API calls.

To run the tests:

```bash
# Run the unit tests
go test -v -run TestRegionFallbackOrdering

# Run the integration test (requires API keys)
go test -v -run TestRegionFallback
```

## Testing Strategy

1. Test with a non-existent region first to force fallback
2. Test with a rate-limited region to trigger error-based fallback
3. Test with a slow region to trigger timeout-based fallback
