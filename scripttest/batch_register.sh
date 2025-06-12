#!/bin/bash

# Configurable variables
NUM_USERS=30         # Number of users to create
BASE_STRING="blah"   # Base string for email, name, and password

for ((i=1; i<=NUM_USERS; i++))
do
  EMAIL="${BASE_STRING}${i}@${BASE_STRING}${i}.com"
  NAME="${BASE_STRING}${i}"
  PASSWORD="${BASE_STRING}${i}"

  curl localhost:3333/register -X POST \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\", \"name\":\"$NAME\", \"password\":\"$PASSWORD\", \"invitationToken\":\"comunals\"}" \
#    -vv
done
