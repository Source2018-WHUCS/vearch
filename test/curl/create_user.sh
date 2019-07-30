
curl -v --user "root:secret" -d "user_name=cb" -d "user_password=1234" -d "privilege=create|alter|drop|truncate|insert|delete|update|select"  -d "user_host=" -d "user_db_list=test|cb|ansj" http://127.0.0.1:8817/manage/user/create
