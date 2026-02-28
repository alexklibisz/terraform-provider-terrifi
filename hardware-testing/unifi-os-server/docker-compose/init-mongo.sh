#!/bin/bash

# Create the unifi database user with dbOwner on the databases that the
# UniFi Network Application requires. This script is bind-mounted into the
# mongo container as a docker-entrypoint-initdb.d script and runs once on
# first start when the data volume is empty.
#
# Init scripts run against the temp mongod (no auth) before it restarts
# with --auth. Create the user in admin so MONGO_AUTHSOURCE=admin works.

if command -v mongosh > /dev/null 2>&1; then
  mongo_cmd="mongosh"
else
  mongo_cmd="mongo"
fi

"${mongo_cmd}" admin <<EOJS
db.createUser({
  user: "unifi",
  pwd: "unifi",
  roles: [
    { db: "unifi", role: "dbOwner" },
    { db: "unifi_stat", role: "dbOwner" },
    { db: "unifi_audit", role: "dbOwner" }
  ]
})
EOJS
