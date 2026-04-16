# proto-cmd

## Instructions

Make sure you create a setup.sh, build.sh, test.sh, and LET_IT_RIP.sh that contain all project setup scripts/commands used - NEVER build/test/run the code in this repo outside of these scripts, NEVER commit or push without running these either. Make them idempotent so that each build.sh can run setup.sh and skip things already set up, each test.sh can run build.sh, each LET_IT_RIP runs test.sh

use go1.26

Port github.com/accretional/runrpc's commander service to here and set it up with tests. Go through its code, impl, docs, etc. and determine if there is anything of interest/concern, report it here in ### Report. Write a quick doc/overview in this file in # Overview
