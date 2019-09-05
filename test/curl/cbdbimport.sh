
cd $GOPATH"/src/github.com/vearch/vearch/tools/cbdbimport"
file=$GOPATH"/bin/vearchimport"
go build -o $file main.go
chmod +x $file
$file --master root:secret@127.0.0.1:8817 --router cb:1234@127.0.0.1:9001 --datafile /tmp/test1.json
