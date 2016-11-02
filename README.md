The Peloton Meta-Framework
===========================


Peloton is Uber's meta-framework for managing, scheduling and
upgrading jobs on Mesos clusters. It has a few unique design priciples
that differentiates itself from other Mesos meta-frameworks:

1. Scalable and high-avaiable multi-master architecture. Unlike other
active-standby frameworks such as Aurora and Marathon, Peloton uses a
multi-master and all-active achitecture where all Peleoton instances
are able to handle both read and write requests concurrently. There is
no single-point of failure or fail-over from standby to active in the
system.

2. Support guranteed rollback when a job upgrade fails. Peloton uses
the latest Mesos resource reservation primitive to upgrade the tasks
on the same nodes as far as possible based on current resource
utilization and scheduling decision. It also reserves the resources on
existing nodes until the upgrade is successful. Otherwise, it will
quickly rollback to previous job configuration on previous nodes
without other dependencies like docker registry.

3. Support persistent or remote volumes for a job. For example,
uConfig or translations data can be mounted into a job instance using
a remote volume referenced by a immutable URL to a
udeploy-replicator. Peloton will use the same job upgrade workflow to
upgrade an uConfig version by simplying changing the URL of a remote
volume.

[Runbook WIP](https://code.uberinternal.com/w/runbooks/peloton/)

## Install

Installations of protoc/proto/protoc-gen-go are required, run bootstrap.sh once so all build dependencies will be installed.
Want to build debian package or docker image ? Follow packaging/README.md

cd $GOPATH

mkdir -p src/code.uber.internal/infra/

git clone gitolite@code.uber.internal:infra/peloton src/code.uber.internal/infra/peloton

cd $GOPATH/src/code.uber.internal/infra/peloton

( run bootstrap.sh only once )

./bootstrap.sh

glide install

make


## Run Peloton master

Before running peloton locally, dependencies like mesos/mysql, need to be set up. Refer to "docker/README.md" on how to
launch dependencies in containers.

PELOTON_HOME=$GOPATH/src/code.uber.internal/infra/peloton

UBER_CONFIG_DIR=$PELOTON_HOME/config/master

UBER_ENVIRONMENT=development

bin/peloton-master

By default, it runs peloton master leader. To run follower, specify flag "--role follower"


## Test Peloton master

1. Create new job via yarpc based go client:

cd $GOPATH/src/code.uber.internal/infra/peloton

bin/peloton-client job create --master http://localhost:5289 --jobid test --yaml test/testjob.yaml


2. Curl into peloton endpoint (To be updated) :

curl -X POST  \
     -H 'content-type: application/json'  \
     -H 'Rpc-Procedure: JobManager.Get'   \
     -H 'Rpc-Service: peloton-master'     \
     -H 'Rpc-Caller: peloton-client'      \
     -H 'Context-TTL-MS: 1000'            \
     -H 'Rpc-Encoding: json'              \
     --data '{"id": {"value": "myjob12345"}}' 	\
    localhost:5289
