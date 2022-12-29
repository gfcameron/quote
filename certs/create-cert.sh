#!/bin/sh
openssl req -new -key server.key -out server.csr -subj "/C=IN/ST=NSW/L=Bengaluru/O=GoLinuxCloud/OU=Org/CN=RootCA"
openssl x509 -req -in server.csr  -CA cacert.pem -CAkey ca.key -out server.crt -CAcreateserial -days 365 -sha256 -extfile server_cert_ext.cnf
cp server.crt certbundle.pem
cat cacert.pem >> certbundle.pem
# Copy the necessary parts to the docker image
mkdir -p ../docker/certs
cp server.key ../docker/certs/server.key
cp certbundle.pem ../docker/certs/certbundle.pem

