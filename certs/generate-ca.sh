#!/bin/sh
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 365 -key ca.key -out cacert.pem -subj "/C=CA/ST=Ontario/L=Kitchener/O=Self/OU=Org/CN=RootCA"
