#!/bin/sh
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 365 -key ca.key -out cacert.pem -subj "/C=IN/ST=NSW/L=Bengaluru/O=GoLinuxCloud/OU=Org/CN=RootCA"
