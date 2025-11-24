#!/bin/bash
# Test all nonstream payloads with real Kiro server
# Uses kiro-sonnet model for all tests

API_KEY="test-api-key-1234567890"
BASE_URL="http://localhost:8317"
TEST_DIR="tests/shared/testdata/nonstream"

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🧪 Testing Kiro Server with All Nonstream Payloads"
echo "=================================================="
echo ""

# Test files - we'll modify the model to kiro-sonnet
files=(
    "openai_format_simple"
    "openai_format"
    "openai_format_with_tools"
    "orignal"
    "orignal_tool_call"
    "orignal_tool_call_no_result"
    "orignal_tool_call_no_tools"
)

successful=0
failed=0

for file in "${files[@]}"; do
    echo -n "Testing $file... "
    
    # Load original payload and modify model to kiro-sonnet
    payload=$(cat "$TEST_DIR/$file.json" | jq '.model = "kiro-sonnet"')
    
    response=$(echo "$payload" | curl -s -w "\n%{http_code}" \
        -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $API_KEY" \
        -d @-)
    
    http_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | head -n-1)
    
    if [ "$http_code" == "200" ]; then
        # Check if response has required fields
        has_id=$(echo "$response_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
        has_choices=$(echo "$response_body" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")
        
        if [ "$has_id" == "yes" ] && [ "$has_choices" == "yes" ]; then
            echo -e "${GREEN}✓ PASS${NC}"
            content=$(echo "$response_body" | jq -r '.choices[0].message.content // "N/A"' | head -c 80)
            finish_reason=$(echo "$response_body" | jq -r '.choices[0].finish_reason')
            echo "  └─ Status: $finish_reason"
            echo "  └─ Content: $content..."
            ((successful++))
        else
            echo -e "${YELLOW}⚠ PARTIAL${NC}"
            echo "  └─ Response missing required fields"
            echo "  └─ Body: $(echo "$response_body" | head -c 200)"
            ((failed++))
        fi
    else
        echo -e "${RED}✗ FAIL${NC}"
        echo "  └─HTTP $http_code"
        error_msg=$(echo "$response_body" | jq -r '.error.message // .error // .' 2>/dev/null | head -c 150)
        echo "  └─ Error: $error_msg"
        ((failed++))
    fi
    echo ""
done

echo "=================================================="
echo -e "Results: ${GREEN}$successful passed${NC}, ${RED}$failed failed${NC}"
echo "Total: $((successful + failed)) tests"

# Show sample response if any passed
if [ $successful -gt 0 ]; then
    echo ""
    echo "Sample successful response:"
    payload=$(cat "$TEST_DIR/openai_format_simple.json" | jq '.model = "kiro-sonnet"')
    echo "$payload" | curl -s \
        -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $API_KEY" \
        -d @- | jq '.' 2>/dev/null | head -30
fi
