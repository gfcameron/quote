#!/bin/sh
#export API_KEY="C227WD9W3LUVKVV9"
#export SYMBOL="MSFT"
#export NDAYS="5"
#export LISTEN_ADDR="127.0.0.1:8443"
#export DOCKER_BUILDKIT=1
docker build --secret id=API_KEY -t quote .