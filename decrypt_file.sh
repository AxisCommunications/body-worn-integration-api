#!/bin/bash

# Argument $1 is the private content key (eg. contentkey.private.pem)
# $2 is the video file to decrypt
# $3 is optional and the output file of the decrypted file. Defaults to decrypted_`basename $2`. Use "-" for stdout.

if [ ! -f "$1" ]; then
	echo "private key (argument 1) does not exist"
	exit 1
fi
if [ ! -f "$2" ]; then
	echo "video file (argument 2) does not exist"
	exit 1
fi
if [ ! -f "$2.key" ]; then
	echo "files wrapper key not found (perhaps not encrypted?)"
	exit 1
fi
outputFile="decrypted_$(basename $2)"
if [ -n "$3" ]; then
	# Hint: "-" works for stdout.
	outputFile="$3"
fi

encryptionKey=$(
	# Extract base64 encoded key value
	grep EncryptedKey $2.key | cut -d'"' -f 4 | \
	# Base 64 decode key
	base64 -d - | \
	# Unwrap key using private key ($1)
	openssl rsautl -decrypt -oaep -inkey $1 -in - | \
	# Convert key to hex
	xxd -p | tr -d '\n')
encryptionIV=$(
	# Extract base64 encoded IV value
	grep ContentEncryptionIV $2.key | cut -d'"' -f 4 | \
	# Base 64 decode IV
	base64 -d - | \
	# Convert IV to hex
	xxd -p | tr -d '\n')

if [ -z "$encryptionKey" ]; then
	echo "failed to decode encryption key"
	exit 2
fi
if [ -z "$encryptionIV" ]; then
	echo "failed to decode encryption iv"
	exit 2
fi

#Decrypt video
openssl enc -d -aes-256-cbc -iv $encryptionIV -in $2 -out $outputFile -K $encryptionKey
exit $?