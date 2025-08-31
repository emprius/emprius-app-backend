# Messaging System Documentation for Frontend Team

## Overview

The Emprius app now includes a comprehensive messaging system that allows users to communicate through different channels. This document provides all the technical details needed to implement the messaging feature in the frontend application.

## Message Types

The system supports three types of messages:

### 1. Private Messages
- **Purpose**: Direct one-on-one communication between users
- **Visibility**: Only visible to sender and recipient
- **Use Case**: Personal conversations, private discussions about tools/bookings

### 2. General Forum Messages
- **Purpose**: Public messages visible to all users
- **Visibility**: All active users can see and respond
- **Use Case**: General announcements, community-wide discussions, public questions

### 3. Community Messages (Infrastructure Ready)
- **Purpose**: Messages within specific communities
- **Visibility**: Only community members can see and participate
- **Use Case**: Community-specific discussions, local announcements
- **Status**: Backend infrastructure is ready, but not yet fully implemented

## Core Concepts

### Messages
- **Immutable**: Once sent, messages cannot be edited or deleted
- **Content**: Text content up to 5,000 characters
- **Images**: Up to 10 images per message (using existing image upload system)
- **Timestamps**: All messages include creation timestamps
- **Read Status**: Track whether each user has read each message

### Conversations
- **Private Conversations**: Automatically created between two users when they exchange messages
- **Participants**: List of users involved in the conversation
- **Metadata**: Last message, message count, unread count, last activity time

### Unread Counts
- **Per-Type Tracking**: Separate counts for private, general forum, and community messages
- **Real-time Updates**: Counts update immediately when messages are sent/read
- **User Profile Integration**: Unread counts are included in user profile API response

## API Endpoints

All endpoints require JWT authentication via the `Authorization: Bearer <token>` header.

### 1. Send Message
```http
POST /messages
Content-Type: application/json
Authorization: Bearer <jwt_token>

{
  "type": "private|general|community",
  "recipientId": "user_id_here",  // Required for private messages
  "communityId": "community_id",  // Required for community messages (when implemented)
  "content": "Message text here",
  "images": ["image_hash_1", "image_hash_2"],  // Optional, max 10 images
  "replyToId": "message_id"  // Optional, for threaded replies (future feature)
}
```

**Response (201 Created):**
```json
{
  "header": {"success": true},
  "data": {
    "id": "message_id",
    "type": "private",
    "senderId": "sender_user_id",
    "senderName": "Sender Name",
    "recipientId": "recipient_user_id",
    "content": "Message text here",
    "images": ["hash1", "hash2"],
    "createdAt": 1693526400,
    "isRead": false
  }
}
```

**Rate Limiting:**
The messaging system implements rate limiting to prevent spam:
- **Private messages**: 30 messages per minute
- **Community messages**: 20 messages per minute  
- **General forum messages**: 10 messages per minute

When rate limits are exceeded, the API returns HTTP 429 (Too Many Requests).

**Validation Rules:**
- Must have either `content` or `images` (or both)
- Content cannot exceed 5,000 characters
- Maximum 10 images per message
- Private messages require `recipientId`
- Community messages require `communityId`
- Cannot send messages to inactive users
- Rate limits apply per user per message type

### 2. Get Messages
```http
GET /messages?type=private&conversationWith=user_id&page=0&pageSize=20
Authorization: Bearer <jwt_token>
```

**Query Parameters:**
- `type`: Filter by message type (`private`, `general`, `community`)
- `conversationWith`: For private messages, specify the other user's ID
- `communityId`: For community messages, specify community ID
- `page`: Page number (0-based, default: 0)
- `pageSize`: Messages per page (default: 16, max: 100)

**Response (200 OK):**
```json
{
  "header": {"success": true},
  "data": {
    "messages": [
      {
        "id": "message_id",
        "type": "private",
        "senderId": "sender_id",
        "senderName": "Sender Name",
        "recipientId": "recipient_id",
        "content": "Message content",
        "images": ["hash1", "hash2"],
        "createdAt": 1693526400,
        "isRead": true
      }
    ],
    "pagination": {
      "current": 0,
      "pageSize": 20,
      "total": 45,
      "pages": 3
    }
  }
}
```

**Notes:**
- Messages are sorted by creation time (newest first)
- `isRead` indicates if the current user has read the message
- Images are returned as hashes that can be used with the existing image API

### 3. Mark Messages as Read

#### Mark Single Message
```http
POST /messages/{messageId}/read
Authorization: Bearer <jwt_token>
```

#### Mark Multiple Messages
```http
POST /messages/read
Content-Type: application/json
Authorization: Bearer <jwt_token>

{
  "messageIds": ["msg_id_1", "msg_id_2", "msg_id_3"]
}
```

#### Mark Entire Conversation as Read
```http
POST /messages/read/conversation
Content-Type: application/json
Authorization: Bearer <jwt_token>

{
  "conversationKey": "private:user_id_1:user_id_2"
}
```

**Response (200 OK):**
```json
{
  "header": {"success": true},
  "data": {
    "success": true,
    "markedCount": 3  // Number of messages marked as read
  }
}
```

### 4. Get Unread Message Counts
```http
GET /messages/unread
Authorization: Bearer <jwt_token>
```

**Response (200 OK):**
```json
{
  "header": {"success": true},
  "data": {
    "total": 15,
    "private": 8,
    "generalForum": 7,
    "communities": {
      "community_id_1": 3,
      "community_id_2": 1
    }
  }
}
```

### 5. Search Messages
```http
GET /messages/search?q=tool%20repair&type=private&page=0&pageSize=20
Authorization: Bearer <jwt_token>
```

**Query Parameters:**
- `q`: Search query string (required, 1-100 characters)
- `type`: Filter by message type (`private`, `general`, `community`) - optional
- `page`: Page number (0-based, default: 0)
- `pageSize`: Messages per page (default: 20, max: 50)

**Response (200 OK):**
```json
{
  "header": {"success": true},
  "data": {
    "messages": [
      {
        "id": "message_id",
        "type": "private",
        "senderId": "sender_id",
        "senderName": "Sender Name",
        "recipientId": "recipient_id",
        "content": "I need help with tool repair techniques",
        "images": [],
        "createdAt": 1693526400,
        "isRead": true
      }
    ],
    "pagination": {
      "current": 0,
      "pageSize": 20,
      "total": 12,
      "pages": 1
    }
  }
}
```

**Notes:**
- Search uses MongoDB full-text search with relevance scoring
- Results are filtered based on user permissions (users only see messages they have access to)
- Private messages: only messages where user is sender or recipient
- Community messages: only messages from communities user belongs to
- General forum messages: visible to all users
- Search is case-insensitive and supports partial word matching

### 6. Get Conversations
```http
GET /conversations?type=private&page=0&pageSize=20
Authorization: Bearer <jwt_token>
```

**Query Parameters:**
- `type`: Filter by conversation type (`private`, `community`)
- `page`: Page number (0-based, default: 0)
- `pageSize`: Conversations per page (default: 16)

**Response (200 OK):**
```json
{
  "header": {"success": true},
  "data": {
    "conversations": [
      {
        "id": "conversation_id",
        "type": "private",
        "participants": [
          {
            "id": "user_id_1",
            "name": "User One",
            "avatarHash": "avatar_hash",
            "rating": 85,
            "active": true
          }
        ],
        "lastMessage": {
          "id": "last_message_id",
          "content": "Last message content",
          "senderId": "sender_id",
          "senderName": "Sender Name",
          "createdAt": 1693526400
        },
        "unreadCount": 3,
        "messageCount": 25,
        "lastMessageTime": 1693526400
      }
    ],
    "pagination": {
      "current": 0,
      "pageSize": 20,
      "total": 5,
      "pages": 1
    }
  }
}
```

## User Profile Integration

The user profile API now includes unread message counts:

```http
GET /profile
Authorization: Bearer <jwt_token>
```

**Response includes:**
```json
{
  "data": {
    "id": "user_id",
    "name": "User Name",
    // ... other profile fields
    "unreadMessageCount": {
      "total": 15,
      "private": 8,
      "generalForum": 7,
      "communities": {
        "community_id": 2
      }
    }
  }
}
```

## Image Handling

Messages can include images using the existing image upload system:

1. **Upload Images First**: Use the existing `POST /images` endpoint to upload images
2. **Get Image Hashes**: The upload response includes image hashes
3. **Include in Message**: Add the hashes to the `images` array in the message request
4. **Display Images**: Use the existing `GET /images/{hash}` endpoint to display images

**Example Flow:**
```javascript
// 1. Upload image
const uploadResponse = await fetch('/images', {
  method: 'POST',
  body: formData,
  headers: { 'Authorization': `Bearer ${token}` }
});
const { hash } = await uploadResponse.json();

// 2. Send message with image
const messageResponse = await fetch('/messages', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`
  },
  body: JSON.stringify({
    type: 'private',
    recipientId: 'user_id',
    content: 'Check out this image!',
    images: [hash]
  })
});
```

## Notification System Integration

The messaging system integrates with the existing notification preferences:

### New Notification Types
- `private_messages`: Notifications for private messages
- `community_messages`: Notifications for community messages
- `general_forum_messages`: Notifications for general forum messages
- `daily_message_digest`: Daily digest of unread messages

### Managing Preferences
Users can enable/disable notifications for each message type using the existing notification preferences API:

```http
POST /profile/notifications
Content-Type: application/json
Authorization: Bearer <jwt_token>

{
  "private_messages": true,
  "community_messages": false,
  "general_forum_messages": true,
  "daily_message_digest": true
}
```

## Frontend Implementation Guidelines

### 1. Real-time Updates
- **Polling Strategy**: Poll `/messages/unread` every 30-60 seconds to update unread counts
- **Message Refresh**: Refresh message lists when new messages are detected
- **Conversation Updates**: Update conversation list when new messages arrive

### 2. Conversation Keys
For private conversations, use this format for conversation keys:
```javascript
function generateConversationKey(userId1, userId2) {
  const sortedIds = [userId1, userId2].sort();
  return `private:${sortedIds[0]}:${sortedIds[1]}`;
}
```

### 3. Message Display
- **Chronological Order**: Display messages in chronological order (oldest first in conversation view)
- **Read Status**: Show read/unread indicators
- **Sender Information**: Display sender name and avatar
- **Timestamps**: Show relative timestamps (e.g., "2 hours ago")

### 4. Unread Count Display
- **Badge Notifications**: Show unread counts as badges on navigation items
- **Total Count**: Display total unread count in main navigation
- **Per-Type Counts**: Show separate counts for private messages and general forum

### 5. Error Handling
- **Network Errors**: Handle connection failures gracefully
- **Validation Errors**: Display user-friendly error messages for validation failures
- **Permission Errors**: Handle cases where users try to message inactive users

### 6. Performance Considerations
- **Pagination**: Always use pagination for message lists
- **Image Loading**: Lazy load images in message threads
- **Caching**: Cache conversation lists and recent messages locally
- **Debouncing**: Debounce typing indicators and auto-save drafts

## Example Frontend Flows

### Sending a Private Message
```javascript
async function sendPrivateMessage(recipientId, content, images = []) {
  try {
    const response = await fetch('/messages', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${userToken}`
      },
      body: JSON.stringify({
        type: 'private',
        recipientId,
        content,
        images
      })
    });
    
    if (response.ok) {
      const result = await response.json();
      // Update UI with new message
      addMessageToConversation(result.data);
      // Clear input
      clearMessageInput();
    } else {
      // Handle error
      showErrorMessage('Failed to send message');
    }
  } catch (error) {
    showErrorMessage('Network error');
  }
}
```

### Loading Conversation Messages
```javascript
async function loadConversationMessages(otherUserId, page = 0) {
  try {
    const response = await fetch(
      `/messages?type=private&conversationWith=${otherUserId}&page=${page}&pageSize=20`,
      {
        headers: { 'Authorization': `Bearer ${userToken}` }
      }
    );
    
    if (response.ok) {
      const result = await response.json();
      displayMessages(result.data.messages);
      updatePagination(result.data.pagination);
    }
  } catch (error) {
    showErrorMessage('Failed to load messages');
  }
}
```

### Marking Messages as Read
```javascript
async function markConversationAsRead(otherUserId) {
  const conversationKey = generateConversationKey(currentUserId, otherUserId);
  
  try {
    await fetch('/messages/read/conversation', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${userToken}`
      },
      body: JSON.stringify({ conversationKey })
    });
    
    // Update unread counts in UI
    updateUnreadCounts();
  } catch (error) {
    console.error('Failed to mark messages as read');
  }
}
```

## Testing Considerations

### Unit Tests
- Test message sending with various content types
- Test pagination and filtering
- Test unread count calculations
- Test conversation key generation

### Integration Tests
- Test complete message flows (send → receive → read)
- Test image upload and display in messages
- Test notification preference updates
- Test error handling scenarios

### User Experience Tests
- Test message delivery and read status updates
- Test conversation list ordering and updates
- Test unread count accuracy
- Test performance with large message volumes

## Security Notes

- All endpoints require JWT authentication
- Users can only see messages they're authorized to view
- Image hashes are validated to ensure images exist
- Input validation prevents XSS and injection attacks
- Rate limiting should be implemented on the frontend to prevent spam

## Future Enhancements

The current implementation provides a solid foundation for future features:

- **Message Reactions**: Like/emoji reactions to messages
- **Message Threading**: Reply to specific messages
- **Advanced Search Filters**: Search by date range, sender, or message type combinations
- **File Attachments**: Support for document and file sharing
- **Voice Messages**: Audio message support
- **Message Encryption**: End-to-end encryption for private messages
- **Push Notifications**: Real-time push notifications for new messages
- **Message Drafts**: Save and restore message drafts
- **Message Forwarding**: Forward messages between conversations

## Support and Questions

For technical questions or issues with the messaging system implementation, please contact the backend team or refer to the complete API documentation in the Swagger specification at `/docs`.
