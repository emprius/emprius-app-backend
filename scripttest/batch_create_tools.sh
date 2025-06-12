#!/bin/bash

# CONFIGURATION
LOGIN_URL="http://localhost:3333/login"
TOOL_CREATE_URL="http://localhost:3333/tools"
EMAIL="kone@kone.com"
PASSWORD="kone@kone.com"
#EMAIL="blah1@blah1.com"
#PASSWORD="blah1"
NUM_TOOLS=200
TOOL_PREFIX="lolazo"

# LOGIN AND RETRIEVE TOKEN
TOKEN=$(curl -s -X POST "$LOGIN_URL" \
  -H "Content-Type: application/json" \
  -d '{"email": "'$EMAIL'", "password": "'$PASSWORD'"}' | jq -r '.data.token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "Login failed. Exiting."
  exit 1
fi

echo "Successfully authenticated. Token: $TOKEN"

#    --argjson location '{"lat": 40.7128, "lng": -74.0060}' \
#latitude	41914916
#longitude	1669922
# CREATE TOOLS IN A LOOP
for i in $(seq 1 $NUM_TOOLS); do
  TOOL_DATA=$(jq -n \
    --arg title "$TOOL_PREFIX $i" \
    --arg desc "Description for Tool $i" \
    --argjson isAvailable true \
    --argjson toolCategory 1 \
    --argjson toolValuation 1000 \
    --argjson height 10 \
    --argjson weight 5 \
    --argjson isNomadic false \
    --argjson maxDistance 0 \
    --argjson cost 50 \
    --argjson location '{"latitude": 41803430, "longitude": 1341133}' \
    '{
      title: $title,
      description: $desc,
      isAvailable: $isAvailable,
      toolCategory: $toolCategory,
      toolValuation: $toolValuation,
      height: $height,
      weight: $weight,
      isNomadic: $isNomadic,
      maxDistance: $maxDistance,
      cost: $cost,
      location: $location,
      images: []
    }')

  echo "Creating Tool $i..."

  RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$TOOL_CREATE_URL" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$TOOL_DATA")

  if [ "$RESPONSE" -eq 200 ]; then
    echo "Tool $i created successfully."
  else
    echo "Failed to create Tool $i. HTTP Status: $RESPONSE"
  fi

done
