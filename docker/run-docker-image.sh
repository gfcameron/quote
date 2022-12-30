#!/bin/sh
docker run -p 8443:8443 \
    -e "API_KEY=C227WD9W3LUVKVV9" \
    -e "SYMBOL=MSFT" \
    -e "NDAYS=5" \
    -v `pwd`/certs:/certs:ro \
    quote