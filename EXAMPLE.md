# Content destination example service

This is an example of how you could implement a content destination for the Axis body worn system.

## Getting started

You need [go](https://golang.org/) installed. Then clone or download this
repository.

From the project folder run `go mod download` to get the dependencies. (Listed
in the `go.mod` file)

## Build

One way is to run

```sh
go build .
```

The `Makefile` is setup to build binaries for Windows, Linux and Mac. To build
them, run:

```sh
make
```

## Run

To install the service you need admin/sudo privileges.

For the rest of the text, Linux is assumed but it works the same for Mac and
Windows.

Navigate to the folder where you have the binary in your terminal.
Run `sudo ./AxisBodyWornSwiftServiceExample_linux-amd64 install` and follow
the steps. Swap *install* with *uninstall* to uninstall the service.

If installation was successful, the service can be started with
`./AxisBodyWornSwiftServiceExample_linux-amd64 start`. Swap *start* with *stop*
to stop the service.

The binary can also be run directly in the terminal by omitting all
arguments. It won't run as a service but it will work even if the service
failed to install due to lacking privileges. A benefit of running in the
terminal is that you can see the server output directly in the terminal.

A `config.json` should now have been created in the storage folder you specified
in the setup wizard. It's used to connect an Axis body worn system controller
to this content destination. You will likely need to create firewall rules
(TCP, port 8080) when using Windows in order to communicate with the body worn
system.

### Run test

```sh
go test
```

## File encryption

During the installation you're first asked to use existing keys, and if
none are selected you're asked if you would like to have the files generated.
If no content encryption keys are selected or generated, content encryption is disabled.

Content encryption requires a public RSA keyfile (PEM encoded, >=1024 bit).
The filename is used as the key ID.
The public key is included in the generated `config.json` file and the private key
is required to be able to decrypt the uploaded content.

Note that on Windows and some Linux distros you need to install OpenSSL
first.

If you run the command where you don't have write permission you won't
get an ouput file. To solve this on Windows you can add `C:\` to the start
of the -out and -in paths.

### Automatically generate content encryption keys

If you do not provide any existing keys, you are given the choice to
generate new ones. Confirm this choice by entering an output directory for the
generated files. If no output directory is selected no keys are generated
and content encryption disabled.

The generated key files are named `contentkey.private.pem` and `contentkey.public.pem`
and located in the selected output directory.

### Manually generate content encryption keys

You can manually generate an RSA keypair using this command:

```sh
openssl genpkey -algorithm RSA -out private_key.pem -pkeyopt rsa_keygen_bits:2048
```

Then extract the public key:

```sh
openssl rsa -pubout -in private_key.pem -out public_key.pem
```

## File decryption

Each video file transfered from the camera should now be accompanied by a
keyfile. In the keyfile there's an attribute `EncryptedKey`. Extract the value
into a file named `wrapped_encryption.key.base64`. It's base 64 encoded so we
need to decode that. Then use the private key to unwrap the encryption key for
the clip, then use that key to decrypt the video.

To play the file, follow the steps below or use
the `play_encrypted` script (make sure you have the `private_key.pem` in the directory you're
executing from).

To decrypt and store the file, use the `decrypt_file.sh` script.

```sh
# Extract key value
grep EncryptedKey videofile.key | cut -d'"' -f 4 > wrapped_encryption.key.base64

# Base 64 decode key
base64 -d wrapped_encryption.key.base64 > wrapped_encryption.key

# Unwrap key
openssl rsautl -decrypt -oaep -inkey private_key.pem -in wrapped_encryption.key -out encryption.key

# Convert key to hex
xxd -p encryption.key | tr -d '\n' > encryption.key.hex

# Extract IV value
grep ContentEncryptionIV videofile.key | cut -d'"' -f 4 > encryption.iv.base64

# Base 64 decode IV
base64 -d encryption.iv.base64 > encryption.iv

# Convert IV to hex
xxd -p encryption.iv | tr -d '\n' > encryption.iv.hex

# Decrypt video
openssl enc -d -aes-256-cbc -iv `cat encryption.iv.hex` -in encrypted_videofile.mkv -out decrypted_videofile.mkv -K `cat encryption.key.hex`

# Play video
vlc decrypted_videofile.mkv
```

## File structure

**Root directory** is the directory which is chosen during installation and is
the storage location for all containers.

**System** is a container that stores all system metadata. It contains a
System.metadata.json object which is the metadata for the container System.
If supported (decided via the corresponding capability), every BWS that
connects to the server creates an empty object with the name `<bwsid>` and a
corresponding bwsid metadata file, named `System.<bwsid>.metadata.json`.

If not running with the `FullStoreAndReadSupport` flag the example server
creates a `Capabilities.json` file.

**Users** is a container that stores all user metadata. It contains a
users.metadata.json object which is the metadata for the container Users. For
every user it creates an empty object with the name `<userid>`. It also
creates user metadata for this object which is named
`Users.<userid>.metadata.json`.

**Devices** has the same structure as the Users container but contains all the
metadata for devices.

For every recording a new container is created. It's named
`<userid>_<deviceid>_date_time` where `userid` is the UUID of the user who was
assigned to the camera when it was recording, and `deviceid` is the ID of the
camera which did the recording.

In every recording container there is one or multiple clips with the naming
`<date>_<time>_<id>.<mkv|mp4>`, along with its corresponding metadata
`<containername>.<clipname>.metadata.json`. If encryption is used, there is
also a `<date>_<time>_<id>.key` file for each clip and a corresponding
`<containername>.<keyname>.metadata.json`.

There is a metadata file for the container which is called
`<containername>.metadata.json`.

There could also be a GNSS track file called
`<date>_<time>_<id>_<bwcid>_gpstrail.json`, along with its corresponding
metadata `<containername>.<gnsstrailname>.metadata.json`.

Furthermore there could be one or several bookmarks with the name
`bookmark_<timestamp>_<ID>`, along with its corresponding metadata
`<containername>.<bookmarkname>.metadata.json`.

To make integration easier, a zero-sized file named `complete` is added
inside the container directory once the `status` metadata attribute is set to
`Complete`. Thereby an integrating application can inotify/watch for that file
to appear instead of polling and parsing the JSON metadata file.

```
Root Directory
├── System
│   ├── <bwsid>
│   ├── System.<bwsid>.metadata.json
│   └── Capabilities.json
├── Users
│   ├── Users.metadata.json
│   ├── userid
│   └── Users.<userid>.metadata.json
├── Devices
│   ├── Devices.metadata.json
│   ├── deviceid
│   └── Devices.<deviceid>.metadata.json
└── containername:<userid>_<deviceid>_date_time
    ├── <containername>.metadata.json
    ├── clipname: <date>_<time>_<id>.mkv
    ├── <containername>.<clipname>.metadata.json
    ├── keyname: <date>_<time>_<id>.key
    ├── <containername>.<keyname>.metadata.json
    ├── bookmarkname: bookmark_<timestamp>_<ID>
    ├── <containername>.<bookmarkname>.metadata.json
    ├── gnsstrailname: <date>_<time>_<id>_<bwcid>_gpstrail.json
    └── <containername>.<gnsstrailname>.metadata.json
```

## GNSSViewerExample

To view any GNSS track associated to a recording run the `GNSSViewerExample` with
the path to the GNSS file as an argument. Once done don't forget to terminate
the program in the terminal, or close the terminal.

## Logs

### Windows: Event Viewer

When running as a service, logs can be found in the Event Viewer.
Expand `Windows logs` and select `Application`. The event source is
`AxisBodyWornSwiftServiceExample`. You can filter the view to only show logs
from this source. In the right sidepanel, `Actions > Application`, click
`Filter current log` and select `AxisBodyWornSwiftServiceExample` in the
`Event sources` dropdown.

### Linux: journalctl

When running as a service, logs can be found with journalctl.

```sh
sudo journalctl -f | grep "AxisBodyWornSwiftServiceExample"
```

### Running in console

When not running as a service, logs are printed directly to the console.

## License

Copyright 2020-2022 Axis Communications AB

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
