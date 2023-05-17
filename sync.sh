#!/bin/bash

set -e

DEST=$1
DIR="/usr/src/tv-led-strip"

rsync -avh -e "ssh -i $HOME/.ssh/rpi.key" --rsync-path="sudo rsync" . $DEST:$DIR
ssh -i ~/.ssh/rpi.key -t $DEST "cd $DIR && /usr/local/go/bin/go install -v && sudo cp tv-led-strip.service /lib/systemd/system && sudo systemctl daemon-reload"
