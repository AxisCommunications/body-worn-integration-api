# Axis Body worn integration API

The Body worn integration API makes it possible to integrate a third-party application as a content destination (CD) to the Body worn system (BWS). The application that implements the API operates as the server, while the BWS is the client that calls the API. The API itself is HTTPS-based and communication with the application is protected by certificates, which need to be configured before the API can be used.

## Terminology

|Term | Description |
|--- |--- |
|`BWM` | Body worn manager |
|`BWS` | Body worn system |
|`BWC` | Body worn camera |
|`CD` | Content destination |

## Overview

### API model

The body worn integration API is a **semantic API**, modelled on top of the [*OpenStack Swift* API](https://wiki.openstack.org/wiki/Swift), an open source *S3*-like object store API. The [technical definition](http://developer.openstack.org/api-ref-objectstorage-v1.html) of the API **is** the Swift API. The main philosophy is that it shall be possible to use a standard Swift object store file server as CD, without any active server process in-between.

Only a small subset of the available Swift API is used in order to create `Containers` and `Objects` and meta data for these. In each API subgroup we specify which methods and attributes that are used and what they mean.

The only subgroup implemented outside of the Swift API is the **Connection API**, which is just a JSON file with configurations.

### Object addressing

The Swift API implements 3 levels that can be used for addressing: `Account`, `Container` and `Object`. The `Account` corresponds to the login specified by a `BlobAPIKey` and `BlobAPIUserName` in the connection file (see below). The `Container` is the top level directory in a file path, the `Object` is the rest of the path and may include multiple slashes.

### Data structure

Each `Recording` from the BWS creates a corresponding `Container` and each `Clip` in a `Recording` creates an `Object` in the `Container` with the video as its content.

A `Recording` consists of:

- `N` video clip objects, `N > 0`
- one keyfile object per video clip, if end to end encryption is enabled
- a location object, if track support is enabled
- one keyfile object per location object, if end to end encryption is enabled
- `M` bookmark files, `M >= 0`, if bookmarks are enabled

Each video clip in the `Recording` corresponds to an `Object` in the `Container`.

### String encoding

All user supplied strings are URL encoded UTF-8 and can include any character except for control characters. Servers are expected to accept this, but may translate characters in their UI if they don't support the full UTF-8 set

## API Subgroups

The Body worn integration API is divided into the following parts:

| API | Usage |
|---|---|
| Connection API | Setup the connection to the content destination. |
| Capability API | Publish supported capabilities from a content destination to a JSON file. |
| Access token API | Upload URI and credentials. |
| Device and user API | Manage devices and users. |
| File upload API | File transfer. |

## API endpoints

The following table lists the used endpoints of the Swift API. All except for the auth endpoint require a valid auth token, while the auth endpoint itself requires both a username and key. This is the subset essential for 3rd party implementations. Everything else such as how to delete and copy objects or containers is not going to be used from the body worn system.

| HTTPS method | Endpoint | Description | Normal responses | Error responses | Detailed Swift documentation |
|---|---|---|---|---|---|
| `Get` | `/auth/v1.0` | Retrieves an auth token. | 200 OK | 401&nbsp;Unauthorized | [Authorization](https://www.swiftstack.com/docs/cookbooks/swift_usage/auth.html) |
| `Get` | `/v1/{account}/`<br>`System/{object}`<sup>1</sup> | Retrieves an object and metadata. | 200 OK | 400 Bad Request<br>401 Unauthorized<br>404 Not Found<br>500 Internal Server Error | [Get object](https://docs.openstack.org/api-ref/object-store/?expanded=#get-object-content-and-metadata) |
| `Put` | `/v1/{account}/`<br>`{container}` | Creates a container | 201 Created<br>202 Accepted | 400 Bad Request<br>401 Unauthorized<br>403 Forbidden<br>404 Not Found<br>500 Internal Server Error<br>503 Service Unavailable | [Create container](https://docs.openstack.org/api-ref/object-store/?expanded=#create-container) |
| `Post` | `/v1/{account}/`<br>`{container}` | Creates or updates the container metadata. | 204&nbsp;No&nbsp;Content | 400 Bad Request<br>401 Unauthorized<br>403 Forbidden<br>404 Not Found<br>500 Internal Server Error | [Update container](https://docs.openstack.org/api-ref/object-store/?expanded=#create-update-or-delete-container-metadata) |
| `Put` | `/v1/{account}/`<br>`{container}/{object}` | Creates an object | 201 Created | 400 Bad Request<br>401 Unauthorized<br>402 Payment Required<br>403 Forbidden<br>404 Not Found<br>500 Internal Server Error<br>503 Service Unavailable<br>507 Insufficient Storage | [Create object](https://docs.openstack.org/api-ref/object-store/?expanded=#create-or-replace-object) |
| `Post` | `/v1/{account}/`<br>`{container}/{object}` | Creates or updates the object metadata, which also deletes the existing metadata. | 202 Accepted | 400 Bad Request<br>401 Unauthorized<br>402 Payment Required<br>403 Forbidden<br>404 Not Found<br>500 Internal Server Error | [Update object](https://docs.openstack.org/api-ref/object-store/?expanded=#create-or-update-object-metadata) |

1. Put and Post is also done on this endpoint, but covered by their respective patterns further down in the table.

### Error response descriptions

| Code | Description |
|---|---|
| 400 Bad Request | Returned when the content destination can't store (and never will be able to store) the recording. It gets status Recording not transferred in AXIS Body Worn Manager, and becomes available for download. Can be used when: the timestamp is out of bounds, the clip has no duration, a user id or device id doesn't exist |
| 401 Unauthorized | Returned by the content destination when user credentials are invalid or auth token missing |
| 402 Payment Required | Returned when attempting to create a BWC or user in the content destination but there are no more licenses. |
| 403 Forbidden | Returned when user permissions are insufficient for a specific resource |
| 404 Not Found | Returned when specific resource is not available |
| 500&nbsp;Internal&nbsp;Server&nbsp;Error | Any other error |
| 503 Service Unavailable | Returned when the content destination is busy and receives too many upload requests from the system controller, but wants the system controller to try again soon. |
| 507 Insufficient Storage | Returned when the content destination is out of disk space/disk quota. |

## Connection API, set up a connection to the content destination

In order to set up a connection, as well as the Body worn system, a JSON connection file needs to be created for the Body worn manager (BWM), which is the web based management application for the BWS. The connection file shall contain all configurations that are required to connect to the content destination, including installation-specific settings such as encryption and the supported container format for the content destination.

Every application that implements the Body worn integration API must be able to provide this file.

### Setup procedure

The connection file is a JSON file with attribute names, value types and lengths, as shown in the template below. The maximum size of the connection file is 64 kB.

### JSON file format

```json
{
  "ConnectionFileVersion": "1.0",
  "SiteName": "<string:64>",
  "ApplicationName": "<string:256>",
  "ApplicationVersion": "<version-string:64>",
  "ContentDestinationAsNTPServer": <bool>,
  "AuthenticationTokenURI": [ "<URI:512>", "<URI:512>", ... x 10 ],
  "HTTPSCertificate": [ "<Base64 encoded X509 Certificate:16k>", "<Base64 encoded X509 Certificate:16k>", ... x 10 ],
  "BlobAPIKey": "<Password:64>",
  "BlobAPIUserName": "<UserName:64>",
  "ContainerType": "<mkv | mp4>",
  "FullStoreAndReadSupport": <bool>,
  "WantEncryption": <bool>,
  "PublicKey": "<Base64 encoded PEM RSA PublicKey:2048>",
  "PublicKeyId": "<string:128>"
}
```

### Attribute details

Some attributes can be modified after the initial setup is completed and some are optional.

| Parameter | Change | Optional | Description |
|---|:---:|:---:|---|
| ConnectionFileVersion | X | |  |
| SiteName | X | |  |
| ApplicationName | X | |  |
| ApplicationVersion | X | |  |
| ContentDestinationAsNTPServer | X | |  |
| AuthenticationTokenURI | X | |  |
| HTTPSCertificate | X | X | Makes the SCU able to verify the server certificate of the CD and create a secure connection with HTTPS. If the certificate can't be validated, the connection setup fails. If no certificate is set, HTTP is used instead. We strongly recommend using HTTPS for all production purposes and only use HTTP for development/debugging.|
| BlobAPIKey | X | |  |
| BlobAPIUserName | X | |  |
| ContainerType | | X | Default value is mkv (optional). |
| FullStoreAndReadSupport | X | X | Default value is false (optional). Only allowed to change from false to true |
| WantEncryption | X | X | Default value is false. Only allowed to change from false to true |
| PublicKey | X | X | Required if WantEncryption is set. No value allowed otherwise. |
| PublicKeyId | X | X | Required if WantEncryption is set. No value allowed otherwise. |

## Capability API, content destination capabilities

The Capability API makes it possible for a connected content destination to publish its supported features. It's the basis for extendability of the API, while still maintaining backward compatibility. The capability settings control visibility of new functionality in the system and the UI. Once a content destination declares that it has support for a capability, it's expected to always have support for it.

The supported capabilities are published by the content destination in a JSON object named `Capability.json` , which shall be located in the `System/` container. To read the object, the BWS does a `GET` request for `System/Capability.json`.

### File format

```json
{
  "Read": {
    "<CAPABILITY1>": true,
    "<CAPABILITY2>": false
  },
  "Store": {
    "<CAPABILITY3>": true,
    "<CAPABILITY4>": false
  },
  "StoreAndRead": {
    "<CAPABILITY5>": true,
    "<CAPABILITY6>": false
  }
}
```

### Implementation details

Please note that all capabilities are considered unsupported if the object is missing or wrongly formatted. This is also true if a capability is not present or set to false.

When responding to a request the Etag header, which contains the md5 hash of the JSON object, must be set, as it's used as a checksum. A typical log error from the SCU if cases where the header is incorrect is `object corrupt`.

### Connection file

The connection file includes the attribute `FullStoreAndReadSupport`. This is typically set for a standard Swift object store content destination, since new capabilities will always work towards a standard Swift implementation. In cases where you want to limit what is stored on a Swift server, don't set the `FullStoreAndReadSupport` attribute. Instead enable the wanted capabilities in the `System/Capability.json` object.

### Capability naming

`Store` is used as prefix when either `POST` or `PUT` support is required, while `Read` is used as prefix when either `GET` and `HEAD` support is required. In cases where both are required, they are added as a prefix, i.e. `StoreRead`.

### Current capabilities

- `StoreReadSystemID`
- `StoreUserIDKey`
- `StoreBookmarks`
- `StoreSignedVideo`
- `StoreGNSSTrackRecording`

For details on each capability, please see the Capability details chapter below

## Access token API, file upload URI and credentials

The Access token API is used by the BWS to retrieve and use a token with the `PUT` and `POST`  methods. OpenStack Swift has support for multiple auth systems, one of them being their own Keystone, which is the method that should be used in the Body worn integration API.

Swift security relies on auth tokens being passed with each request. A token is retrieved when the BWS sends both an `Auth-Key` and `X-Auth-User` in the header of a `GET` request to the `BaseURL`, which is supplied by the JSON connection file. The content destination must then respond with an `X-Auth-Token` and the `X-Storage-Url`, which are then used during the upload.

## Device and User API, manage devices and users

This API is used when one of the following actions are taken by an admin:

- When a user is registered in the BWM. An object named with the `UserID` of the user is then created in the `Users/` container. The nice name of the user is stored as metadata on the object.

- When a user's nice name is updated, the metadata for that user object is updated.

- When a BWC is registered in the BWM. An object named with the `BWCSerialNumber` of the BWC is then created in the `Devices/` container. The nice name of the device is stored as metadata on the object.

- When a BWC's nice name is updated, the metadata for that camera object is updated.

A recording carries the essential information to map it to a `UserID` and a `BWCSerialNumber`. If the information in the recording doesn't match existing users or devices, a `400 Bad Request` error response should be sent. If the user has been disabled in the content destination, it's advised to re-enable it to receive the content and then disable it again.

## File Upload API, file transfer from BWS to CD

The File upload API is used when the Body worn system uploads a recording created in a BWC. Doing this creates a corresponding recording container, which gets a name according to this template: `<UserID>_<BWCSerialNumber>_<TriggerOnTime>`.

Every clip in the recording corresponds to an object in the container, where the clips are named `<StartTime>_<RecordingID>.<ContainerType>`. Please note that `RecordingID` is a short, random number for the recording that shouldn't be relied upon for identification purposes.

### Overview of Container and Object structure

**System** is a container that stores all system metadata. For
every connected BWS an object named with the `<bwsid>` is created.

**Users** is a container that stores all user metadata. For
every user an object named with the `<userid>` is created.

**Devices** is a container that stores all device metadata. For
every device an object named with the `<deviceid>` is created.

### Recordings

For every recording a new container is created. It's named as
`<userid>_<deviceid>_date_time` where "userid" is the UUID of the user who was
assigned to the camera when it was recording, and "deviceid" is the ID of the
camera which did the recording.

In every recording container there's one or multiple clips.
There could also be a GNSS track object and one or several bookmark objects.

The `status` metadata attribute on the recording container is set to
`Complete` when the BWS is done transferring all data for the recording. No more requests are done on the container from the BWS after this.

```
Account
├── System
│   ├── <bwsid>
│   └── Capabilities.json
├── Users
│   └── <userid>
├── Devices
│   └── <deviceid>
└── recordingname:<userid>_<deviceid>_<date>_<time>
    ├── clipname: <date>_<time>_<id>.mkv
    ├── keyname: <date>_<time>_<id>.key
    ├── bookmarkname: bookmark_<timestamp>_<ID>
    └── gnsstrailname: <date>_<time>_<id>_<bwcid>_gpstrail.json
```

### Clip metadata details

Clip metadata is sent as an HTTP header in the requests. All metadata is presented as strings, and time is specified in UTC.

All metadata header keys have the prefix `X-Object-Meta` or `X-Container-Meta` as `X-<type>-Meta-Key` in the HTTP request. All characters are lowercase except the first letter as well as any letter that follows a hyphen. The values are presented in the URL-encoded UTF-8 format, where characters outside of US ASCII and reserved HTTP characters are `%XX` encoded. Please note that string attributes can't be larger than 32 bytes except when noted, such as the device name, user name and location attributes, which can be up to 64 bytes. Strings supplied by the user may include any character except control characters. Most applications are expected to accept this, but they may sometimes have to translate the characters in their individual user interface in cases where they don't support the full UTF-8 set.

#### Recording container metadata

The name of the container is `<UserID>_<BWCSerialNumber>_<TriggerOnTime>`.

| Key | Type | Description |
|---|---|---|
| BWCSerialNumber | String | The serial number of the device that captures the video in the container. |
| SCUSerialNumber | String | The serial number of the device that received the recording from the BWC. |
| FirmwareVersion | String | The firmware version of the BWC for when the video was recorded. |
| UserID | String | The user that was assigned to the device when the video was recorded. |
| TriggerOn | String | Why the camera started a recording. |
| TriggerOff | String | Why the camera stopped a recording. |
| TriggerOnTime | String | The time when the trigger was issued, epoch UTC. |
| TriggerOnTimeISO | String | The time when the trigger was issued, RFC3339 format (UTC). |
| TriggerOnLocation | String[64] | `<lat><long><accuracy><epoch>` |
| TriggerOffTime | String | The time when the recording was stopped, epoch UTC. |
| TriggerOffTimeISO | String | The time when the recording was stopped, RFC3339 format (UTC). |
| TriggerOffLocation | String[64] | `<lat><long><accuracy><epoch>` |
| BWCModel | String | The BWC model that was used when recording. |
| Status | String | Transferring\|Complete: The status of the Container. When the container metadata becomes Complete, there won't be any more updates or uploads to either the container or metadata. |

#### Video object metadata

The name of the object is `<StartTime>_<RecordingID>.<ContainerType>`.

| Key | Type | Description |
|---|---|---|
| StartTime | String | The time when the clip started, epoch UTC. |
| StartTimeISO | String | The time when the clip started, RFC3339 format (UTC) |
| StopTime | String | The time when the clip ended, epoch UTC. |
| StopTimeISO | String | The time when the clip ended, RFC3339 format (UTC). |
| ContainerType | String | The file type of the clip (mkv or mp4) |

#### Key object metadata

The name of the object is `<StartTime>_<RecordingID>.key`.

#### Bookmark object metadata

The name of the object is `bookmark_<Timestamp>_<ID>`.

This object is stored only if `StoreBookmarks` capability is set.

The content of the object is the free text description, in UTF-8. As there is a timestamp, multiple descriptions can be added. This object can also work as a bookmark. Then the content and all metadata is empty, except for the timestamp.

| Key | Type | Description |
|---|---|---|
| CategoryID | String | The category ID. |
| CategoryName | String | The string for this category. |
| Tags | String | A semicolon separated string of tags. |
| StartTime | String | Timestamp for the description in RFC3339 (UTC). If not set by the user, it starts at the beginning of the recording. |
| EndTime | String | Timestamp for the description in RFC3339 (UTC). If not set by the user there is no key. |

#### Location object meta-data

The name of the object is `<date>_<time>_<RecordingID>_<bwcid>_gpstrail.json`.

This object is stored only if the `StoreGNSSTrackRecording` capability is set.

| Key | Type | Description |
|---|---|---|
| FileType | String | Always 'json'. See below for file format |

#### Location file format

```json
{
  "CoordinateEntries": [
    {
      "LocationWKT": "POINT(13.221184 55.718409)",
      "SecondsFromStart": 64.0,
      "Timestamp": "2022-08-23T11:48:10Z"
    },
    {
      "LocationWKT": "POINT(13.220701 55.718702)",
      "SecondsFromStart": 82.0,
      "Timestamp": "2022-08-23T11:48:28Z"
    },
    {
      "LocationWKT": "POINT(13.221424 55.718778)",
      "SecondsFromStart": 92.0,
      "Timestamp": "2022-08-23T11:48:38Z"
    },
    {
      "LocationWKT": "POINT(13.221766 55.718877)",
      "SecondsFromStart": 102.0,
      "Timestamp": "2022-08-23T11:48:48Z"
    },
    {
      "LocationWKT": "POINT(13.221725 55.718951)",
      "SecondsFromStart": 113.0,
      "Timestamp": "2022-08-23T11:48:59Z"
    }
  ]
}
```

Where `SecondsFromStart` is seconds since `StartTime` of the recording. `StartTime` might not be the same as `TriggerOnTime` for example if running with pre-buffer.

We recommend using the `TriggerOnLocation` in the recording metadata if you want to include information about where the user started the recording.

`LocationWKT` is defined as `Point(x, y)`, where `x` is the longitude and `y` is the latitude.

#### System container metadata

The container name is `System`.

This container is created if the `StoreReadSystemID` capability is set.

#### System object metadata

Object name is `<SystemID>`, a UUID.

This object is stored only if the `StoreReadSystemID` capability is set.

Multiple system objects can exist if several discrete BWS are connected to the same content destination.

All data is stored by the body worn system. Only the `SystemName` is intended for end user consumption, and can be updated at any time from the body worn system. The `ConnectionID` is set when the object is created and never changes.

| Key | Type | Description |
|---|---|---|
| ConnectionId | String[100] | The connection ID for this system. |
| SystemName | String[100] | The nice name of the body worn system installation. |

#### User container metadata

The container name is `Users`.

#### User object metadata

Object name is `<UserUUID>`, an internal UUID.

| Key | Type | Description |
|---|---|---|
| Active | String | True/False |
| Name | String[100] | Nice name of the user, is not unique |
| UserID | String[100] | A user supplied ID, is empty or unique. Only stored if `StoreUserIDKey` capability is set. |

>**Note**
>
>Name is not unique. We recommend that you display Name (UserID).

#### Devices container metadata

The container name is `Devices`.

#### Device object metadata

Object name is `<BWCSerialNumber>`.

| Key | Type | Description |
|---|---|---|
| Active | String | True/False |
| Name | String[100] | The nice name of the device. |
| Model | String | Device model (e.g. W100) |

## Object encryption

### Encrypt

In the connection file, a key for end to end content encryption can be set with the `PublicKey` and `PublicKeyId` parameters. The `WantEncryption` parameter should also be set to ´true´.

The corresponding private key is required to be able to decrypt the uploaded content. The `PublicKeyId` is written to the video key object which is stored together with the encrypted video clip. GNSS track objects are encrypted in the same manner. The CD can use the ID to know which key to use for decryption.

If `WantEncryption` parameter is set and `PublicKey` is not set, the BWS will display an error. If `WantEncryption` parameter is not set, content encryption is disabled, and the `PublicKey` is ignored.

If encryption has been enabled for a system it can not be turned off and a configuration for such a system is considered invalid and is rejected if it would render encryption disabled.

### Decrypt

When encryption is enabled, each content object is accompanied by a
keyfile object. In the keyfile object there's an attribute ´EncryptedKey´, which holds the content encryption key, wrapped with the public key.

See the documentation for the example server for more details on how to perform the decryption.

## Capability details

### SystemID

When a BWS is configured for the first time, it's also assigned a System ID. This ID is stored on the BWS and also stored over the API on the content destination. It makes it possible to check that the system is communicating with the expected endpoint instance. System ID is a way to uniquely bind a content destination to a body worn system. As long as the System ID matches it's allowed to change config almost completely and still be able to ensure that the body worn system is talking to the same content destination. Without System ID it will be possible to swap to any other CD which opens up for different kinds of vulnerabilities or robustness issues. It's therefore strongly adviced to implement the System ID capability. Enabled using the `StoreReadSystemID` capability.

### User ID

If the `StoreUserIDKey` capability is set, the BWM shows a field for a user supplied ID for each user. The ID shall be empty or unique. Stored as a metadata key on the object of the user.

### Bookmarks

Enabled using the `StoreBookmarks` capability. The `tags` key has a semicolon separated list of `key:value` pairs as value. Implementers should be prepared for unknown keys and values in this list.

Example of a `tags` `key:value` string: `"tags": "TriggerOn:Button;SomeTag:SomeValue"`

### Signed Video

Signed Video capability declares support for receiving video with embedded signature data. If the content destination can display signed videos and is able to export the signed video without tampering with the original it can activate signed video using the `StoreSignedVideo` capability.
Signed video can be used to verify the integrity and authenticity of a recording. See
https://www.axis.com/developer-community/signed-video for more information.

### GNSS track

If the content destination can retrieve and display GNSS track data from a recording, it can activate it using the `StoreGNSSTrackRecording` capability. The GNSS data comes as a separate JSON object included in the recording container, other formats may be added later. See the chapter on **Location object format** for details.

## License

Copyright 2020-2023 Axis Communications AB

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
