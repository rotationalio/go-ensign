#!/bin/bash
# This script helps us generate the Go source code from protocol buffer files where the
# protocol buffers are stored in the github.com/rotationalio/ensign repository. If the
# script cannot find the repository it exits without error after printing a warning that
# the other repository must be cloned first.

# Find the rotationalio/ensign repository using the $GOPATH or a path relative to the
# generate.sh script if $GOPATH is not set.
if [[ -z "${GOPATH}" ]]; then
    DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
    PROTOS=$(realpath "$DIR/../ensign/proto")
else
    PROTOS="${GOPATH}/src/github.com/rotationalio/ensign/proto"
fi

# If the protos directory does not exist, exit with a warning
if [[ ! -d $PROTOS ]]; then
    echo "cannot find ${PROTOS}"
    echo "must clone the github.com/rotationalio/ensign repo before generating protocol buffers"
    exit 0
fi

# Generate the protocol buffers
protoc -I ${PROTOS} \
    --go_out=. \
    --go_opt=module="github.com/rotationalio/go-ensign" \
    --go_opt=Mmimetype/v1beta1/mimetype.proto="github.com/rotationalio/go-ensign/mimetype/v1beta1;mimetype" \
    mimetype/v1beta1/mimetype.proto

# protoc -I ${PROTOS} \
#     --go_out="${GOPATH}/src" --go-grpc_out="${GOPATH}/src" \
#     --go_opt=module="github.com/rotationalio/go-ensign" \
#     --go_opt=Mmimetype/v1beta1/mimetype.proto="github.com/rotationalio/go-ensign/mimetype/v1beta1;mimetype" \
#     --go_opt=Mapi/v1beta1/ensign.proto="github.com/rotationalio/go-ensign/api/v1beta1;api" \
#     --go_opt=Mapi/v1beta1/event.proto="github.com/rotationalio/go-ensign/api/v1beta1;api" \
#     --go_opt=Mapi/v1beta1/topic.proto="github.com/rotationalio/go-ensign/api/v1beta1;api" \
#     --go-grpc_opt=Mapi/v1beta1/ensign.proto="github.com/rotationalio/go-ensign/api/v1beta1;api" \
#     --go-grpc_opt=Mapi/v1beta1/event.proto="github.com/rotationalio/go-ensign/api/v1beta1;api" \
#     --go-grpc_opt=Mapi/v1beta1/topic.proto="github.com/rotationalio/go-ensign/api/v1beta1;api" \
#     api/v1beta1/ensign.proto \
#     api/v1beta1/event.proto \
#     api/v1beta1/topic.proto