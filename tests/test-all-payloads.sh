#!/bin/bash
# Test all nonstream payloads with real Kiro server
# Separates OpenAI Chat Completions format tests from Anthropic Messages format tests

API_KEY="${API_KEY:-test-api-key-1234567890}"
BASE_URL="${BASE_URL:-http://localhost:8317}"
MODEL="${MODEL:-kiro-sonnet}"
TEST_DIR="tests/shared/testdata/nonstream"

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "🧪 Testing Kiro Server with All Nonstream Payloads"
echo "=================================================="
echo ""

successful=0
failed=0

# ============================================================================
# Test OpenAI Chat Completions Format (/v1/chat/completions)
# ============================================================================
echo -e "${BLUE}📋 Testing OpenAI Chat Completions Format${NC}"
echo "Endpoint: /v1/chat/completions"
echo ""

openai_files=(
    "openai_format_simple"
    "openai_format"
    "openai_format_with_tools"
)

for file in "${openai_files[@]}"; do
    echo -n "Testing $file... "
    
    # Load original payload and modify model to kiro-sonnet
    payload=$(cat "$TEST_DIR/$file.json" | jq --arg model "$MODEL" '.model = $model')
    
    response=$(echo "$payload" | curl -s -w "\n%{http_code}" \
        -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $API_KEY" \
        -d @-)
    
    http_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | head -n-1)
    
    if [ "$http_code" == "200" ]; then
        # Validate OpenAI Chat Completions format
        has_id=$(echo "$response_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
        has_choices=$(echo "$response_body" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")
        has_message=$(echo "$response_body" | jq -e '.choices[0].message' > /dev/null 2>&1 && echo "yes" || echo "no")
        
        if [ "$has_id" == "yes" ] && [ "$has_choices" == "yes" ] && [ "$has_message" == "yes" ]; then
            echo -e "${GREEN}✓ PASS${NC}"
            content=$(echo "$response_body" | jq -r '.choices[0].message.content // "N/A"' | head -c 80)
            finish_reason=$(echo "$response_body" | jq -r '.choices[0].finish_reason')
            echo "  └─ Finish: $finish_reason"
            echo "  └─ Content: $content..."
            ((successful++))
        else
            echo -e "${YELLOW}⚠ PARTIAL${NC}"
            echo "  └─ Response missing required OpenAI fields (id/choices/message)"
            echo "  └─ Body: $(echo "$response_body" | head -c 200)"
            ((failed++))
        fi
    else
        echo -e "${RED}✗ FAIL${NC}"
        echo "  └─ HTTP $http_code"
        error_msg=$(echo "$response_body" | jq -r '.error.message // .error // .' 2>/dev/null | head -c 150)
        echo "  └─ Error: $error_msg"
        ((failed++))
    fi
    echo ""
done

# ============================================================================
# Test Anthropic Messages Format (/v1/messages)
# ============================================================================
echo -e "${BLUE}📋 Testing Anthropic Messages Format${NC}"
echo "Endpoint: /v1/messages"
echo ""

anthropic_files=(
    "orignal"
    "orignal_tool_call"
    "orignal_tool_call_no_result"
    "orignal_tool_call_no_tools"
)

for file in "${anthropic_files[@]}"; do
    echo -n "Testing $file... "
    
    # Load original payload and modify model to kiro-sonnet
    payload=$(cat "$TEST_DIR/$file.json" | jq --arg model "$MODEL" '.model = $model')
    
    response=$(echo "$payload" | curl -s -w "\n%{http_code}" \
        -X POST "$BASE_URL/v1/messages" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $API_KEY" \
        -H "anthropic-version: 2023-06-01" \
        -d @-)
    
    http_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | head -n-1)
    
    if [ "$http_code" == "200" ]; then
        # Validate Anthropic Messages format
        has_id=$(echo "$response_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
        has_content=$(echo "$response_body" | jq -e '.content' > /dev/null 2>&1 && echo "yes" || echo "no")
        has_role=$(echo "$response_body" | jq -e '.role' > /dev/null 2>&1 && echo "yes" || echo "no")
        
        if [ "$has_id" == "yes" ] && [ "$has_content" == "yes" ] && [ "$has_role" == "yes" ]; then
            echo -e "${GREEN}✓ PASS${NC}"
            # Extract content (handle both text and tool_use types)
            content=$(echo "$response_body" | jq -r '.content[0].text // .content[0].type // "N/A"' | head -c 80)
            stop_reason=$(echo "$response_body" | jq -r '.stop_reason')
            echo "  └─ Stop: $stop_reason"
            echo "  └─ Content: $content..."
            ((successful++))
        else
            echo -e "${YELLOW}⚠ PARTIAL${NC}"
            echo "  └─ Response missing required Anthropic fields (id/content/role)"
            echo "  └─ Body: $(echo "$response_body" | head -c 200)"
            ((failed++))
        fi
    else
        echo -e "${RED}✗ FAIL${NC}"
        echo "  └─ HTTP $http_code"
        error_msg=$(echo "$response_body" | jq -r '.error.message // .error // .' 2>/dev/null | head -c 150)
        echo "  └─ Error: $error_msg"
        ((failed++))
    fi
    echo ""
done

# ============================================================================
# Summary
# ============================================================================
echo "=================================================="
echo -e "Results: ${GREEN}$successful passed${NC}, ${RED}$failed failed${NC}"
echo "Total: $((successful + failed)) tests"
echo ""

# Show sample responses if any passed
if [ $successful -gt 0 ]; then
    echo "Sample Responses:"
    echo ""
    
    # Sample OpenAI Chat Completions response
    echo -e "${BLUE}OpenAI Chat Completions Format:${NC}"
    payload=$(cat "$TEST_DIR/openai_format_simple.json" | jq --arg model "$MODEL" '.model = $model')
    echo "$payload" | curl -s \
        -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $API_KEY" \
        -d @- | jq '.' 2>/dev/null | head -20
    
    echo ""
    echo -e "${BLUE}Anthropic Messages Format:${NC}"
    payload=$(cat "$TEST_DIR/orignal.json" | jq --arg model "$MODEL" '.model = $model')
    echo "$payload" | curl -s \
        -X POST "$BASE_URL/v1/messages" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $API_KEY" \
        -H "anthropic-version: 2023-06-01" \
        -d @- | jq '.' 2>/dev/null | head -20
fi

exit $failed
