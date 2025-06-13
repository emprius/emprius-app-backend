#!/bin/bash

# =============================================================================
# CONFIGURATION VARIABLES
# =============================================================================

# API Configuration
BASE_URL="http://localhost:3333"
LOGIN_URL="$BASE_URL/login"
REGISTER_URL="$BASE_URL/register"
COMMUNITIES_URL="$BASE_URL/communities"
TOOLS_URL="$BASE_URL/tools"
USERS_URL="$BASE_URL/users"
DEFAULT_INVITATION_TOKEN="comunals"

# Admin User Configuration
ADMIN_EMAIL="admin@admin.com"
ADMIN_PASSWORD="admin@admin.com"
ADMIN_NAME="Admin User"

# User Configuration
NUM_USERS=100
USER_PREFIX="testuser"
USER_DOMAIN="example.com"

# Community Configuration
NUM_COMMUNITIES=100
COMMUNITY_PREFIX="testcommunity"

# Tool Configuration
NUM_TOOLS=100
TOOL_PREFIX="testtool"

# Invitation Configuration
NUM_INVITED_USERS=100  # How many users to invite to communities

# Location Configuration (Barcelona coordinates in microdegrees)
DEFAULT_LATITUDE=41803430
DEFAULT_LONGITUDE=1341133

# =============================================================================
# HELPER FUNCTIONS
# =============================================================================

# Function to register a user
register_user() {
    local email="$1"
    local password="$2"
    local name="$3"
    
    local register_data=$(jq -n \
        --arg email "$email" \
        --arg password "$password" \
        --arg name "$name" \
        --arg invitationToken "$DEFAULT_INVITATION_TOKEN" \
        --argjson latitude "$DEFAULT_LATITUDE" \
        --argjson longitude "$DEFAULT_LONGITUDE" \
        '{
            email: $email,
            invitationToken: $invitationToken,
            name: $name,
            community: "testCommunity",
            password: $password,
            location: {
                latitude: $latitude,
                longitude: $longitude
            }
        }')
    
    local response=$(curl -s -w "%{http_code}" -X POST "$REGISTER_URL" \
        -H "Content-Type: application/json" \
        -d "$register_data")
    
    local http_code="${response: -3}"
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        return 0
    else
        return 1
    fi
}

# Function to login user and return token
login_user() {
    local email="$1"
    local password="$2"
    
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

# Function to get user token (register if needed, then login)
get_user_token() {
    local email="$1"
    local password="$2"
    local name="$3"

    # Try to login first
    local token=$(login_user "$email" "$password")
    if [ -n "$token" ]; then
        echo "$token"
        return 0
    fi

    # If login failed, try to register and then login
    if register_user "$email" "$password" "$name"; then
        sleep 1
        token=$(login_user "$email" "$password")
        if [ $? -eq 0 ]; then
            echo "$token"
            return 0
        fi
    fi
    
    return 1
}

# Function to create a community
create_community() {
    local token="$1"
    local name="$2"
    local description="$3"
    
    local community_data=$(jq -n \
        --arg name "$name" \
        --arg description "$description" \
        --argjson latitude "$DEFAULT_LATITUDE" \
        --argjson longitude "$DEFAULT_LONGITUDE" \
        '{
            name: $name,
            description: $description,
            location: {
                latitude: $latitude,
                longitude: $longitude
            }
        }')
    
    local response=$(curl -s -X POST "$COMMUNITIES_URL" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d "$community_data")
    
    local community_id=$(echo "$response" | jq -r '.data.id // .id // empty')
    if [ -n "$community_id" ] && [ "$community_id" != "null" ]; then
        echo "$community_id"
        return 0
    else
        return 1
    fi
}

# Function to get user ID by email
get_user_id() {
    local token="$1"
    local email="$2"
    
    # Extract username from email for search term
    local username=$(echo "$email" | cut -d'@' -f1)
    
    local response=$(curl -s -X GET "$USERS_URL?page=0&term=$username" \
        -H "Authorization: Bearer $token")
    
    local user_id=$(echo "$response" | jq -r --arg email "$email" '.data.users[] | select(.email == $email) | .id // empty')
    if [ -n "$user_id" ] && [ "$user_id" != "null" ]; then
        echo "$user_id"
        return 0
    else
        return 1
    fi
}

# Function to invite user to community
invite_user_to_community() {
    local token="$1"
    local community_id="$2"
    local user_id="$3"
    
    local response=$(curl -s -w "%{http_code}" -X POST "$COMMUNITIES_URL/$community_id/members/$user_id" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json")
    
    local http_code="${response: -3}"
    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        return 0
    else
        return 1
    fi
}

# Function to get pending invites for a user
get_pending_invites() {
    local token="$1"
    
    local response=$(curl -s -X GET "$COMMUNITIES_URL/invites" \
        -H "Authorization: Bearer $token")
    
    local invite_ids=$(echo "$response" | jq -r '.data[]?.id // empty')
    if [ -n "$invite_ids" ]; then
        echo "$invite_ids"
        return 0
    else
        return 1
    fi
}

# Function to accept an invitation
accept_invitation() {
    local token="$1"
    local invite_id="$2"
    
    local response=$(curl -s -w "%{http_code}" -X PUT "$COMMUNITIES_URL/invites/$invite_id" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d '{"status": "ACCEPTED"}')
    
    local http_code="${response: -3}"
    if [ "$http_code" -eq 200 ]; then
        return 0
    else
        return 1
    fi
}

# Function to create a tool
create_tool() {
    local token="$1"
    local title="$2"
    local description="$3"
    
    local tool_data=$(jq -n \
        --arg title "$title" \
        --arg description "$description" \
        --argjson isAvailable true \
        --argjson mayBeFree true \
        --argjson askWithFee false \
        --argjson toolCategory 1 \
        --argjson toolValuation 1000 \
        --argjson height 10 \
        --argjson weight 5 \
        --argjson cost 50 \
        --argjson latitude "$DEFAULT_LATITUDE" \
        --argjson longitude "$DEFAULT_LONGITUDE" \
        '{
            title: $title,
            description: $description,
            isAvailable: $isAvailable,
            mayBeFree: $mayBeFree,
            askWithFee: $askWithFee,
            toolCategory: $toolCategory,
            toolValuation: $toolValuation,
            height: $height,
            weight: $weight,
            cost: $cost,
            location: {
                latitude: $latitude,
                longitude: $longitude
            },
            images: []
        }')
    
    local response=$(curl -s -X POST "$TOOLS_URL" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d "$tool_data")
    
    local tool_id=$(echo "$response" | jq -r '.data.id // .id // empty')
    if [ -n "$tool_id" ] && [ "$tool_id" != "null" ]; then
        echo "$tool_id"
        return 0
    else
        return 1
    fi
}

# Function to update tool to share with community
update_tool_communities() {
    local token="$1"
    local tool_id="$2"
    local community_id="$3"
    
    local tool_data=$(jq -n \
        --arg community_id "$community_id" \
        '{
            communities: [$community_id]
        }')
    
    local response=$(curl -s -w "%{http_code}" -X PUT "$TOOLS_URL/$tool_id" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d "$tool_data")
    
    local http_code="${response: -3}"
    if [ "$http_code" -eq 200 ]; then
        return 0
    else
        return 1
    fi
}

# =============================================================================
# MAIN EXECUTION
# =============================================================================

echo "Starting complete batch setup script"
echo "Configuration:"
echo "  - Admin: $ADMIN_EMAIL"
echo "  - Users: $NUM_USERS"
echo "  - Communities: $NUM_COMMUNITIES"
echo "  - Tools: $NUM_TOOLS"
echo "  - Invited users: $NUM_INVITED_USERS"
echo "  - Base URL: $BASE_URL"
echo ""

# Step 1: Admin User Setup
echo "=== Step 1: Admin User Setup ==="

ADMIN_TOKEN=$(get_user_token "$ADMIN_EMAIL" "$ADMIN_PASSWORD" "$ADMIN_NAME")
if [ $? -ne 0 ]; then
    echo "❌ Failed to setup admin user. Exiting."
    exit 1
fi
echo "✅ Admin user authenticated successfully"

# Step 2: Create Regular Users
echo "=== Step 2: Creating $NUM_USERS Users ==="

declare -a USER_EMAILS=()
declare -a USER_PASSWORDS=()
declare -a USER_NAMES=()
SUCCESSFUL_USERS=0

for i in $(seq 1 $NUM_USERS); do
    EMAIL="${USER_PREFIX}${i}@${USER_DOMAIN}"
    PASSWORD="${USER_PREFIX}${i}"
    NAME="${USER_PREFIX}${i}"
    
    if register_user "$EMAIL" "$PASSWORD" "$NAME"; then
        USER_EMAILS+=("$EMAIL")
        USER_PASSWORDS+=("$PASSWORD")
        USER_NAMES+=("$NAME")
        SUCCESSFUL_USERS=$((SUCCESSFUL_USERS + 1))
    fi
    
    # Progress indicator
    if [ $((i % 10)) -eq 0 ]; then
        echo "Created $i/$NUM_USERS users..."
    fi
done

echo "✅ Successfully created $SUCCESSFUL_USERS/$NUM_USERS users"

# Step 3: Create Communities
echo "=== Step 3: Creating $NUM_COMMUNITIES Communities ==="

declare -a COMMUNITY_IDS=()
SUCCESSFUL_COMMUNITIES=0

for i in $(seq 1 $NUM_COMMUNITIES); do
    COMMUNITY_NAME="${COMMUNITY_PREFIX}${i}"
    COMMUNITY_DESC="Description for $COMMUNITY_NAME"
    
    community_id=$(create_community "$ADMIN_TOKEN" "$COMMUNITY_NAME" "$COMMUNITY_DESC")
    if [ $? -eq 0 ]; then
        COMMUNITY_IDS+=("$community_id")
        SUCCESSFUL_COMMUNITIES=$((SUCCESSFUL_COMMUNITIES + 1))
    fi
    
    # Progress indicator
    if [ $((i % 10)) -eq 0 ]; then
        echo "Created $i/$NUM_COMMUNITIES communities..."
    fi
done

echo "✅ Successfully created $SUCCESSFUL_COMMUNITIES/$NUM_COMMUNITIES communities"

# Step 4: Invite Users to Communities
echo "=== Step 4: Inviting Users to First Community ==="

SUCCESSFUL_INVITATIONS=0
INVITE_COUNT=0

# Limit invitations to available users
MAX_INVITES=$(( NUM_INVITED_USERS < SUCCESSFUL_USERS ? NUM_INVITED_USERS : SUCCESSFUL_USERS ))

# Use the first community for all invitations
FIRST_COMMUNITY_ID=""
if [ ${#COMMUNITY_IDS[@]} -gt 0 ]; then
    FIRST_COMMUNITY_ID="${COMMUNITY_IDS[0]}"
else
    echo "❌ No communities available for invitations. Skipping Step 4."
    MAX_INVITES=0
fi

for i in $(seq 0 $((MAX_INVITES - 1))); do
    if [ $i -lt ${#USER_EMAILS[@]} ]; then
        EMAIL="${USER_EMAILS[$i]}"
        
        # Get user ID
        user_id=$(get_user_id "$ADMIN_TOKEN" "$EMAIL")
        if [ $? -eq 0 ]; then
            if invite_user_to_community "$ADMIN_TOKEN" "$FIRST_COMMUNITY_ID" "$user_id"; then
                SUCCESSFUL_INVITATIONS=$((SUCCESSFUL_INVITATIONS + 1))
            fi
        fi
        
        INVITE_COUNT=$((INVITE_COUNT + 1))
        
        # Progress indicator
        if [ $((INVITE_COUNT % 10)) -eq 0 ]; then
            echo "Sent $INVITE_COUNT/$MAX_INVITES invitations to first community..."
        fi
    fi
done

echo "✅ Successfully sent $SUCCESSFUL_INVITATIONS/$MAX_INVITES invitations"

# Step 5: Accept Community Invitations
echo "=== Step 5: Accepting Community Invitations ==="

SUCCESSFUL_ACCEPTANCES=0

for i in $(seq 0 $((MAX_INVITES - 1))); do
    if [ $i -lt ${#USER_EMAILS[@]} ] && [ $i -lt ${#USER_PASSWORDS[@]} ] && [ $i -lt ${#USER_NAMES[@]} ]; then
        EMAIL="${USER_EMAILS[$i]}"
        PASSWORD="${USER_PASSWORDS[$i]}"
        NAME="${USER_NAMES[$i]}"
        
        # Login as user
        user_token=$(get_user_token "$EMAIL" "$PASSWORD" "$NAME")
        if [ $? -eq 0 ]; then
            # Get pending invites
            invite_ids=$(get_pending_invites "$user_token")
            if [ $? -eq 0 ]; then
                # Accept each invite
                for invite_id in $invite_ids; do
                    if accept_invitation "$user_token" "$invite_id"; then
                        SUCCESSFUL_ACCEPTANCES=$((SUCCESSFUL_ACCEPTANCES + 1))
                        break  # Accept only one invite per user
                    fi
                done
            fi
        fi
        
        # Progress indicator
        if [ $(((i + 1) % 10)) -eq 0 ]; then
            echo "Processed $((i + 1))/$MAX_INVITES invitation acceptances..."
        fi
    fi
done

echo "✅ Successfully accepted $SUCCESSFUL_ACCEPTANCES invitations"

# Step 6: Create Tools and Share to First Community
echo "=== Step 6: Creating $NUM_TOOLS Tools and Sharing to First Community ==="

SUCCESSFUL_TOOLS=0
SUCCESSFUL_SHARES=0

# Use first community if available
FIRST_COMMUNITY_ID=""
if [ ${#COMMUNITY_IDS[@]} -gt 0 ]; then
    FIRST_COMMUNITY_ID="${COMMUNITY_IDS[0]}"
fi

for i in $(seq 1 $NUM_TOOLS); do
    TOOL_TITLE="${TOOL_PREFIX}${i}"
    TOOL_DESC="Description for $TOOL_TITLE"
    
    tool_id=$(create_tool "$ADMIN_TOKEN" "$TOOL_TITLE" "$TOOL_DESC")
    if [ $? -eq 0 ]; then
        SUCCESSFUL_TOOLS=$((SUCCESSFUL_TOOLS + 1))
        
        # Share with first community if available
        if [ -n "$FIRST_COMMUNITY_ID" ]; then
            if update_tool_communities "$ADMIN_TOKEN" "$tool_id" "$FIRST_COMMUNITY_ID"; then
                SUCCESSFUL_SHARES=$((SUCCESSFUL_SHARES + 1))
            fi
        fi
    fi
    
    # Progress indicator
    if [ $((i % 10)) -eq 0 ]; then
        echo "Created $i/$NUM_TOOLS tools..."
    fi
done

echo "✅ Successfully created $SUCCESSFUL_TOOLS/$NUM_TOOLS tools"
if [ -n "$FIRST_COMMUNITY_ID" ]; then
    echo "✅ Successfully shared $SUCCESSFUL_SHARES/$SUCCESSFUL_TOOLS tools with first community"
fi

# Final Summary
echo ""
echo "=== Final Summary ==="
echo "✅ Admin user: 1/1"
echo "✅ Users created: $SUCCESSFUL_USERS/$NUM_USERS"
echo "✅ Communities created: $SUCCESSFUL_COMMUNITIES/$NUM_COMMUNITIES"
echo "✅ Invitations sent: $SUCCESSFUL_INVITATIONS/$MAX_INVITES"
echo "✅ Invitations accepted: $SUCCESSFUL_ACCEPTANCES/$MAX_INVITES"
echo "✅ Tools created: $SUCCESSFUL_TOOLS/$NUM_TOOLS"
if [ -n "$FIRST_COMMUNITY_ID" ]; then
    echo "✅ Tools shared with first community: $SUCCESSFUL_SHARES/$SUCCESSFUL_TOOLS"
fi

if [ $SUCCESSFUL_USERS -eq $NUM_USERS ] && [ $SUCCESSFUL_COMMUNITIES -eq $NUM_COMMUNITIES ] && [ $SUCCESSFUL_TOOLS -eq $NUM_TOOLS ]; then
    echo ""
    echo "🎉 Script completed successfully!"
    exit 0
else
    echo ""
    echo "⚠️  Script completed with some issues. Check the summary above."
    exit 1
fi
