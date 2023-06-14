#!/bin/bash

team_name="main"
team_display_name="Mattermost AI"
channel_name="ai"
channel_display_name="AI"
user_name="root"
user_password="$(openssl rand -base64 14)"

echo -e "Setting up Mattermost with ...\n Team name: $team_name\n Team display name: $team_display_name\n Channel name: $channel_name\n Channel display name: $channel_display_name"

echo "Initializing Mattermost for demo. This will take about 30 seconds..."
sleep 35

docker exec mattermost mmctl --local team create --display-name $team_display_name --name $team_name
docker exec mattermost mmctl --local channel create --team $team_name --display-name "$channel_display_name" --name $channel_name

#echo -e "Initializing your admin account ...\n\n User name: $user_name\n User password: $user_password\n"
docker exec mattermost mmctl --local user create --username $user_name --password $user_password --email $user_name@$team_name.com --system-admin --email-verified
docker exec mattermost mmctl --local team users add $team_name $user_name
docker exec mattermost mmctl --local channel users add $team_name:$channel_name $user_name

#docker exec -e MM_SERVICESETTINGS_SITEURL=http://localhost:8065 -e MM_ADMIN_USERNAME=$user_name -e MM_ADMIN_PASSWORD=$user_password mattermost /bin/sh make deploy /mattermost-ai-plugin

echo -e "\n===========================\n\n  FIRST, DO THIS COMMAND: MM_SERVICESETTINGS_SITEURL=http://localhost:8065 MM_ADMIN_USERNAME=root MM_ADMIN_PASSWORD=$user_password make deploy"
echo -e "\n===========================\n\n  THEN LOG IN TO MATTERMOST AT $(gp url 8065)\n\n        username:  $user_name\n        password:  $user_password\n\n  THEN CONFIGURE THE PLUGIN & ENJOY!\n\n"
