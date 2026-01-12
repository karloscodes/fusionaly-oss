#!/bin/bash

# This script tests the website_goals setting in the database

DATABASE_PATH="storage/fusionaly-development.db"

echo "Testing website_goals setting in $DATABASE_PATH"

# Check if setting exists
if [ -f "$DATABASE_PATH" ]; then
  echo "Database file found"
  
  # Check if the setting exists
  SETTING_VALUE=$(sqlite3 "$DATABASE_PATH" "SELECT value FROM settings WHERE key='website_goals';")
  
  if [ -n "$SETTING_VALUE" ]; then
    echo "Setting found with value: $SETTING_VALUE"
    
    # Try to format the JSON to see if it's valid
    echo "Attempting to format JSON:"
    echo "$SETTING_VALUE" | python3 -m json.tool
    
    if [ $? -eq 0 ]; then
      echo "JSON is valid"
    else
      echo "JSON is invalid"
      
      # If invalid, create a new empty valid JSON
      echo "Creating new empty valid JSON"
      NEW_JSON='{"goals":{}}'
      
      # Update the setting in the database
      sqlite3 "$DATABASE_PATH" "UPDATE settings SET value='$NEW_JSON' WHERE key='website_goals';"
      
      echo "Setting updated. New value:"
      sqlite3 "$DATABASE_PATH" "SELECT value FROM settings WHERE key='website_goals';"
    fi
  else
    echo "Setting not found"
    
    # Create the setting with a valid empty JSON
    echo "Creating setting with empty valid JSON"
    JSON='{"goals":{}}'
    
    sqlite3 "$DATABASE_PATH" "INSERT INTO settings (key, value, created_at, updated_at) VALUES ('website_goals', '$JSON', datetime('now'), datetime('now'));"
    
    echo "Setting created. Value:"
    sqlite3 "$DATABASE_PATH" "SELECT value FROM settings WHERE key='website_goals';"
  fi
else
  echo "Database file not found at $DATABASE_PATH"
fi
