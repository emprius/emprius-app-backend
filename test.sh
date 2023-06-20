#!/bin/bash

REGISTERTOKEN="comunals"
RND=$RANDOM
MAIL="pepe$RND@botika.cat"
PASSW="pepebotika$RND"
NAME="PepeBotika$RND"
avatar="iVBORw0KGgoAAAANSUhEUgAABAEAAAFBCAYAAAAG47bTAAAACXBIWXMAAC4jAAAuIwF4pT92AAA5nWlUWHRYTUw6Y29tLmFkb2JlLnhtcAAAAAAAPD94cGFja2V0IGJlZ2luPSLvu78iIGlkPSJX"
HOST="${HOST:-http://localhost:3333}"

set -x 

echo "=> register $MAIL"
curl -s $HOST/register -X POST -d '{"email":"'$MAIL'", "name":"'$NAME'", "password":"'$PASSW'", "invitationToken":"'$REGISTERTOKEN'"}' | jq .

echo "=> login with $PASSW"
jwt=$(curl -s $HOST/login -X POST -d '{"email":"'$MAIL'", "password":"'$PASSW'"}' | jq .data.token | tr -d \")
jwt="Authorization: BEARER $jwt"

echo "=> jwt is $jwt"

echo "=> profile for $NAME"
curl -s $HOST/profile -H "$jwt" | jq .

echo "=> info"
curl -s $HOST/info | jq .

echo "=> update profile"
curl -s $HOST/profile -X POST -H "$jwt" -d '{"location":{ "latitude":42202259,"longitude":1815044}, "community":"Karabanchel" }' | jq .

echo "=> update profile with avatar"
curl -s $HOST/profile -X POST -H "$jwt" -d '{"avatar":"'$avatar'"}' | jq .

echo "=> get image avatar"
image=$(curl -s $HOST/profile -H "$jwt" | jq .data.avatarHash | tr -d \")
curl -s $HOST/images/$image -H "$jwt"

echo "=> list users"
curl -s $HOST/users -H "$jwt" | jq .

echo "=> add a new tool"
curl -s $HOST/tools -X POST -H "$jwt" -H 'Content-Type: application/json' -d '{
  "title": "Hammer",
  "description": "A useful tool",
  "mayBeFree": true,
  "askWithFee": false,
  "cost": 10,
  "category": 1,
  "estimatedValue": 20,
  "height": 30,
  "weight": 40,
  "location": {
   "latitude":42202259,
   "longitude":1815044
  }  
}' | jq .

echo "=> get tools owned by the user"
tool_id=$(curl -s $HOST/tools -H "$jwt" | jq ".data.tools[].id")

echo "=> modify a tool"
curl -s $HOST/tools/$tool_id -X PUT -H "$jwt" -H 'Content-Type: application/json' -d '{
  "description": "New Description",
  "cost": 20,
  "category": 2
}' | jq .


echo "=> search a tool"
curl $HOST/tools/search -X GET -H "$jwt"  -H 'Content-Type: application/json' \
-d '{
    "categories": [1, 2],
    "maxCost": 100,
    "distance": 20000
}' | jq .


set +x
