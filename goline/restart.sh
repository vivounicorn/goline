pid=`pgrep goline`

kill $pid

nohup ./bin/goline conf/settings.conf &