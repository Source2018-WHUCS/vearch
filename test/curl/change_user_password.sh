
curl -v --user "root:secret"  -d "user_name=cb" -d "user_password=1234" http://127.0.0.1:8817/manage/user/passwd/update | python -m json.tool
