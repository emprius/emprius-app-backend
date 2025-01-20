#!/bin/bash

REGISTERTOKEN="comunals"
RND=$RANDOM
MAIL="pepe$RND@botika.cat"
PASSW="pepebotika$RND"
NAME="PepeBotika$RND"
avatar="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
HOST="${HOST:-http://localhost:3333}"

set -x 

echo "=> register $MAIL"
curl -s $HOST/register -X POST -d '{"email":"'$MAIL'", "name":"'$NAME'", "password":"'$PASSW'", "invitationToken":"'$REGISTERTOKEN'"}' | jq .

echo "=> login with $PASSW"
curl -s $HOST/login -X POST -d '{"email":"'$MAIL'", "password":"'$PASSW'"}' | jq .
jwt=$(curl -s $HOST/login -X POST -d '{"email":"'$MAIL'", "password":"'$PASSW'"}' | jq .data.token | tr -d \")
jwt="Authorization: BEARER $jwt"

echo "=> jwt is $jwt"

echo "=> profile for $NAME"
curl -s $HOST/profile -H "$jwt" | jq .
# Store user's ObjectID for later use
user_id=$(curl -s $HOST/profile -H "$jwt" | jq -r .data.id)

echo "=> info"
curl -s $HOST/info | jq .

echo "=> update profile"
curl -s $HOST/profile -X POST -H "$jwt" -d '{"location":{ "latitude":42202259,"longitude":1815044}, "community":"Karabanchel" }' | jq .

echo "=> upload avatar image (should be fine using the same image)"
avatarHash=$(curl -s $HOST/images -X POST -H "$jwt" -H 'Content-Type: application/json' -d '{"content":"'$avatar'"}' | jq .data.hash | tr -d \")

echo "=> update profile with avatar"
curl -s $HOST/profile -X POST -H "$jwt" -d '{"avatar":"'$avatar'"}' | jq .

read

echo "=> get image avatar"
image=$(curl -s $HOST/profile -H "$jwt" | jq .data.avatarHash | tr -d \")

curl -s $HOST/images/$image -H "$jwt" | jq .

echo "=> list users"
curl -s $HOST/users -H "$jwt" | jq .

read

echo "=> add a new tool"
curl -s $HOST/tools -X POST -H "$jwt" -H 'Content-Type: application/json' -d '{
  "title": "Hammer",
  "description": "A useful tool",
  "mayBeFree": true,
  "askWithFee": false,
  "cost": 10,
  "images": [],
  "transportOptions": [1, 2],
  "category": 1,
  "location": {
    "latitude": 42202259,
    "longitude": 1815044
  },
  "estimatedValue": 20,
  "height": 30,
  "weight": 40
}' | jq .

echo "=> get tools owned by the user"
curl -s $HOST/tools -H "$jwt" | jq .
tool_id=$(curl -s $HOST/tools -H "$jwt" | jq -r ".data.tools[].id")

echo "=> get tools by user ID"
curl -s $HOST/tools/user/$user_id -H "$jwt" | jq .

echo "=> modify a tool"
curl -s $HOST/tools/$tool_id -X PUT -H "$jwt" -H 'Content-Type: application/json' -d '{
  "description": "New Description",
  "cost": 20,
  "category": 2
}' | jq .

echo "=> search a tool"
curl -s $HOST/tools/search -X GET -H "$jwt" -H 'Content-Type: application/json' -d '{
  "categories": [1, 2],
  "maxCost": 100,
  "distance": 20000,
  "mayBeFree": true
}' | jq .

echo "=> create a booking request"
curl -s $HOST/bookings -X POST -H "$jwt" -H 'Content-Type: application/json' -d '{
  "toolId": "'$tool_id'",
  "startDate": '$(date -d "+1 day" +%s)',
  "endDate": '$(date -d "+2 days" +%s)',
  "contact": "'$MAIL'",
  "comments": "I need this tool for a project"
}' | jq .

echo "=> get booking requests for my tools"
curl -s $HOST/bookings/requests -H "$jwt" | jq .

echo "=> get my booking petitions"
curl -s $HOST/bookings/petitions -H "$jwt" | jq .

# Get the booking ID from the petitions response
booking_id=$(curl -s $HOST/bookings/petitions -H "$jwt" | jq -r '.data[0].id')

echo "=> get specific booking details"
curl -s $HOST/bookings/$booking_id -H "$jwt" | jq .

echo "=> accept a booking petition"
curl -s $HOST/bookings/petitions/$booking_id/accept -X POST -H "$jwt" | jq .

echo "=> mark booking as returned"
curl -s $HOST/bookings/$booking_id/return -X POST -H "$jwt" | jq .

echo "=> get pending ratings"
curl -s $HOST/bookings/rates -H "$jwt" | jq .

echo "=> submit a rating"
curl -s $HOST/bookings/rates -X POST -H "$jwt" -H 'Content-Type: application/json' -d '{
  "bookingId": "'$booking_id'",
  "rating": 5
}' | jq .

set +x
