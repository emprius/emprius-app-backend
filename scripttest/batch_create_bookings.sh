#!/bin/bash

# =============================================================================
# CONFIGURATION VARIABLES
# =============================================================================

# API Configuration
BASE_URL="http://localhost:3333"
LOGIN_URL="$BASE_URL/login"
REGISTER_URL="$BASE_URL/register"
TOOL_CREATE_URL="$BASE_URL/tools"
BOOKING_CREATE_URL="$BASE_URL/bookings"
RATING_ENDPOINT="$BASE_URL/bookings"

# User Configuration
USER1_EMAIL="kone@kone.com"
USER1_PASS="kone@kone.com"
USER1_NAME="Tool Owner"
USER2_EMAIL="blah1@blah1.com"
USER2_PASS="blah1@blah1.com"
USER2_NAME="Tool Renter"

# Booking Configuration
TOTAL_BOOKINGS=100
PENDING_RATINGS_COUNT=100
SUBMIT_RATINGS_COUNT=100  # Modify this value as needed

# Default invitation token (from test utils)
DEFAULT_INVITATION_TOKEN="comunals"

# Location Configuration (New York coordinates in microdegrees)
LATITUDE=40712800  # 40.7128 * 1000000
LONGITUDE=-74006000  # -74.0060 * 1000000

# Tool Configuration
TOOL_NAME="Test Tool for Bookings"
TOOL_DESCRIPTION="A test tool for creating multiple bookings and ratings"
TOOL_CATEGORY=1
TOOL_VALUATION=1000
TOOL_HEIGHT=10
TOOL_WEIGHT=5
TOOL_COST=50
TOOL_MAX_DISTANCE=0

# =============================================================================
# HELPER FUNCTIONS
# =============================================================================

# Function to attempt login
login_user() {
    local email="$1"
    local password="$2"
    local user_name="$3"

    local response=$(curl -s -X POST "$LOGIN_URL" \
        -H "Content-Type: application/json" \
        -d "{\"email\": \"$email\", \"password\": \"$password\"}")
    
    local token=$(echo "$response" | jq -r '.data.token // .token // empty')
    if [ -n "$token" ] && [ "$token" != "null" ]; then
        echo "$token"
        return 0
    else
        return 1
    fi
}

# Function to register a new user
register_user() {
    local email="$1"
    local password="$2"
    local name="$3"
    
    local register_data=$(jq -n \
        --arg email "$email" \
        --arg password "$password" \
        --arg name "$name" \
        --arg invitationToken "$DEFAULT_INVITATION_TOKEN" \
        --argjson latitude "$LATITUDE" \
        --argjson longitude "$LONGITUDE" \
        '{
            email: $email,
            invitationToken: $invitationToken,
            name: $name,
            community: "testCommunity",
            password: $password,
            location: {
                latitude: $latitude,
                longitude: $longitude
            },
            tokens: 1000
        }')
    
    local response=$(curl -s -w "%{http_code}" -X POST "$REGISTER_URL" \
        -H "Content-Type: application/json" \
        -d "$register_data")
    
    local http_code="${response: -3}"
    local body="${response%???}"
    
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        return 0
    else
        return 1
    fi
}

# Function to get user token (login or register + login)
get_user_token() {
    local email="$1"
    local password="$2"
    local name="$3"

    # Try to login first
    local token=$(login_user "$email" "$password" "$name")
    if [ -n "$token" ]; then
        echo "$token"
        return 0
    fi

    # If login failed, try to register and then login
    if register_user "$email" "$password" "$name"; then
        # Wait a moment for registration to complete
        sleep 1
        
        # Try login again
        token=$(login_user "$email" "$password" "$name")
        if [ $? -eq 0 ]; then
            echo "$token"
            return 0
        fi
    fi
    
    log_error "Failed to get token for user: $email"
    return 1
}

# Function to create a tool
create_tool() {
    local token="$1"
    
    local tool_data=$(jq -n \
        --arg title "$TOOL_NAME" \
        --arg desc "$TOOL_DESCRIPTION" \
        --argjson mayBeFree true \
        --argjson askWithFee false \
        --argjson toolCategory "$TOOL_CATEGORY" \
        --argjson toolValuation "$TOOL_VALUATION" \
        --argjson height "$TOOL_HEIGHT" \
        --argjson weight "$TOOL_WEIGHT" \
        --argjson latitude "$LATITUDE" \
        --argjson longitude "$LONGITUDE" \
        '{
            title: $title,
            description: $desc,
            mayBeFree: $mayBeFree,
            askWithFee: $askWithFee,
            toolCategory: $toolCategory,
            toolValuation: $toolValuation,
            height: $height,
            weight: $weight,
            location: {
                latitude: $latitude,
                longitude: $longitude
            }
        }')
    
    local response=$(curl -s -X POST "$TOOL_CREATE_URL" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d "$tool_data")
    
    local tool_id=$(echo "$response" | jq -r '.data.id // .id // empty')
    
    if [ -n "$tool_id" ] && [ "$tool_id" != "null" ]; then
        echo "$tool_id"
        return 0
    else
        log_error "Failed to create tool"
        log_error "Response: $response"
        return 1
    fi
}

# Function to create a booking
create_booking() {
    local token="$1"
    local tool_id="$2"
    local start_date="$3"
    local end_date="$4"
    local booking_num="$5"
    
    local booking_data=$(jq -n \
        --arg toolId "$tool_id" \
        --argjson startDate "$start_date" \
        --argjson endDate "$end_date" \
        --arg contact "test-contact-$booking_num@example.com" \
        --arg comments "Test booking #$booking_num for tool $tool_id" \
        '{
            toolId: $toolId,
            startDate: $startDate,
            endDate: $endDate,
            contact: $contact,
            comments: $comments
        }')
    
    local response=$(curl -s -X POST "$BOOKING_CREATE_URL" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d "$booking_data")
    
    local booking_id=$(echo "$response" | jq -r '.data.id // .id // empty')
    
    if [ -n "$booking_id" ] && [ "$booking_id" != "null" ]; then
        echo "$booking_id"
        return 0
    else
        log_error "Failed to create booking #$booking_num"
        log_error "Response: $response"
        return 1
    fi
}

# Function to update booking status
update_booking_status() {
    local token="$1"
    local booking_id="$2"
    local status="$3"
    
    local status_data=$(jq -n --arg status "$status" '{status: $status}')
    
    local response=$(curl -s -X PUT "$BASE_URL/bookings/$booking_id" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d "$status_data")
    
    local updated_status=$(echo "$response" | jq -r '.data.bookingStatus // .bookingStatus // empty')
    
    if [ "$updated_status" = "$status" ]; then
        return 0
    else
        log_error "Failed to update booking $booking_id to status $status"
        log_error "Response: $response"
        return 1
    fi
}

# Function to generate date range (returns start and end timestamps)
generate_date_range() {
    local days_from_now="$1"
    local duration_days="$2"
    
    local start_date=$(date -d "+$days_from_now days" +%s)
    local end_date=$(date -d "+$((days_from_now + duration_days)) days" +%s)
    
    echo "$start_date $end_date"
}

# =============================================================================
# MAIN EXECUTION
# =============================================================================

echo "Starting batch booking creation script"
echo "Configuration:"
echo "  - Total bookings: $TOTAL_BOOKINGS"
echo "  - Pending ratings: $PENDING_RATINGS_COUNT"
echo "  - Base URL: $BASE_URL"

# Step 1: Get tokens for both users
echo "=== Step 1: User Authentication ==="

TOKEN1=$(get_user_token "$USER1_EMAIL" "$USER1_PASS" "$USER1_NAME")
echo $TOKEN1
if [ $? -ne 0 ]; then
    log_error "Failed to get token for USER1. Exiting."
    exit 1
fi
echo "$USER1_EMAIL token: $TOKEN1"

TOKEN2=$(get_user_token "$USER2_EMAIL" "$USER2_PASS" "$USER2_NAME")
if [ $? -ne 0 ]; then
    log_error "Failed to get token for USER2. Exiting."
    exit 1
fi
echo "$USER2_EMAIL token: $TOKEN2"

echo "Both users authenticated successfully"

# Step 2: Create tool
echo "=== Step 2: Tool Creation ==="

TOOL_ID=$(create_tool "$TOKEN1")
if [ $? -ne 0 ]; then
    log_error "Failed to create tool. Exiting."
    exit 1
fi

# Step 3: Create bookings
echo "=== Step 3: Creating $TOTAL_BOOKINGS Bookings ==="

declare -a BOOKING_IDS=()
SUCCESSFUL_BOOKINGS=0

for i in $(seq 1 $TOTAL_BOOKINGS); do
    # Generate date range (each booking starts i days from now, lasts 1 day)
    dates=$(generate_date_range "$i" 1)
    start_date=$(echo $dates | cut -d' ' -f1)
    end_date=$(echo $dates | cut -d' ' -f2)
    
    booking_id=$(create_booking "$TOKEN2" "$TOOL_ID" "$start_date" "$end_date" "$i")
    if [ $? -eq 0 ]; then
        BOOKING_IDS+=("$booking_id")
        SUCCESSFUL_BOOKINGS=$((SUCCESSFUL_BOOKINGS + 1))
        
        # Progress indicator
        if [ $((i % 10)) -eq 0 ]; then
            echo "Created $i/$TOTAL_BOOKINGS bookings..."
        fi
    else
        log_error "Failed to create booking #$i"
    fi
done

echo "Successfully created $SUCCESSFUL_BOOKINGS/$TOTAL_BOOKINGS bookings"

# Step 4: Process bookings for pending ratings
echo "=== Step 4: Creating $PENDING_RATINGS_COUNT Pending Ratings ==="

if [ ${#BOOKING_IDS[@]} -lt $PENDING_RATINGS_COUNT ]; then
    log_error "Not enough successful bookings (${#BOOKING_IDS[@]}) to create $PENDING_RATINGS_COUNT pending ratings"
    PENDING_RATINGS_COUNT=${#BOOKING_IDS[@]}
    echo "Adjusting to create $PENDING_RATINGS_COUNT pending ratings"
fi

SUCCESSFUL_RATINGS=0

for i in $(seq 0 $((PENDING_RATINGS_COUNT - 1))); do
    booking_id="${BOOKING_IDS[$i]}"
    
    echo "Processing booking $((i + 1))/$PENDING_RATINGS_COUNT (ID: $booking_id)"
    
    # Accept the booking (USER1 accepts USER2's request)
    if update_booking_status "$TOKEN1" "$booking_id" "ACCEPTED"; then
        echo "  ✓ Accepted booking $booking_id"
        
        # Mark as returned (USER1 marks tool as returned)
        if update_booking_status "$TOKEN1" "$booking_id" "RETURNED"; then
            echo "  ✓ Marked booking $booking_id as returned (pending rating)"
            SUCCESSFUL_RATINGS=$((SUCCESSFUL_RATINGS + 1))
        else
            log_error "  ✗ Failed to mark booking $booking_id as returned"
        fi
    else
        log_error "  ✗ Failed to accept booking $booking_id"
    fi
done

# Step 5: Submit ratings for pending bookings

echo "=== Step 5: Submitting $SUBMIT_RATINGS_COUNT Ratings ==="

if [ "$SUBMIT_RATINGS_COUNT" -gt "$PENDING_RATINGS_COUNT" ]; then
    echo "⚠️  SUBMIT_RATINGS_COUNT ($SUBMIT_RATINGS_COUNT) is greater than PENDING_RATINGS_COUNT ($PENDING_RATINGS_COUNT). Adjusting to $PENDING_RATINGS_COUNT."
    SUBMIT_RATINGS_COUNT=$PENDING_RATINGS_COUNT
fi


RATINGS_SUBMITTED=0

for i in $(seq 0 $((SUBMIT_RATINGS_COUNT - 1))); do
    booking_id="${BOOKING_IDS[$i]}"

    echo "Submitting rating $((i + 1))/$SUBMIT_RATINGS_COUNT for Booking ID: $booking_id"

    # Construct rating payload
    rating_data=$(jq -n \
        --argjson rating $(( (RANDOM % 5) + 1 )) \
        --arg comment "Automated test rating for booking $booking_id" \
        '{ rating: $rating, comment: $comment }')

    # Submit rating
    response=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$RATING_ENDPOINT/$booking_id/ratings" \
        -H "Authorization: Bearer $TOKEN2" \
        -H "Content-Type: application/json" \
        -d "$rating_data")

    if [ "$response" -eq 200 ] || [ "$response" -eq 201 ]; then
        echo "  ✓ Rating submitted for booking $booking_id"
        RATINGS_SUBMITTED=$((RATINGS_SUBMITTED + 1))
    else
        echo "  ✗ Failed to submit rating for booking $booking_id (HTTP $response)"
    fi
done



# Step 6: Summary
echo "=== Summary ==="
echo "✓ Users authenticated: 2/2"
echo "✓ Tools created: 1/1 (ID: $TOOL_ID)"
echo "✓ Bookings created: $SUCCESSFUL_BOOKINGS/$TOTAL_BOOKINGS"
echo "✓ Pending ratings created: $SUCCESSFUL_RATINGS/$PENDING_RATINGS_COUNT"
echo "✓ Ratings submitted: $RATINGS_SUBMITTED/$SUBMIT_RATINGS_COUNT"

if [ $SUCCESSFUL_BOOKINGS -eq $TOTAL_BOOKINGS ] && [ $SUCCESSFUL_RATINGS -eq $PENDING_RATINGS_COUNT ]; then
    echo "🎉 Script completed successfully!"
    exit 0
else
    echo "⚠️  Script completed with some issues. Check the logs above."
    exit 1
fi

