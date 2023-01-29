# Update and install the cloud formation

## Build the containers and publish them to your docker hub

```
VMCLARITY_TOOLS_BASE=<your vmclarity tools base image> DOCKER_REGISTRY=<your docker hub> make push-docker
e.g.:
VMCLARITY_TOOLS_BASE=idanfrim/vmclarity-tools-base DOCKER_TAG=new-tag DOCKER_REGISTRY=idanfrim make push-docker
```

## Update installation/aws/VMClarity.cfn

Update the cloud formation with the pushed docker images, a diff for example:

```
@@ -123,7 +123,7 @@ Resources:
                     DATABASE_DRIVER=LOCAL
                     BACKEND_REST_ADDRESS=__BACKEND_REST_ADDRESS__
                     BACKEND_REST_PORT=8888
-                    SCANNER_CONTAINER_IMAGE=tehsmash/vmclarity-cli:dc2d75a10e5583e97f516be26fcdbb484f98d5c3
+                    SCANNER_CONTAINER_IMAGE=idanfrim/vmclarity-cli:new-tag
                   - JobImageID: !FindInMap
                       - AWSRegionArch2AMI
                       - !Ref "AWS::Region"
@@ -145,13 +145,13 @@ Resources:
                 ExecStartPre=-/usr/bin/docker stop %n
                 ExecStartPre=-/usr/bin/docker rm %n
                 ExecStartPre=/usr/bin/mkdir -p /opt/vmclarity
-                ExecStartPre=/usr/bin/docker pull tehsmash/vmclarity-backend:dc2d75a10e5583e97f516be26fcdbb484f98d5c3
+                ExecStartPre=/usr/bin/docker pull idanfrim/vmclarity-backend:new-tag
                 ExecStart=/usr/bin/docker run \
                   --rm --name %n \
                   -p 0.0.0.0:8888:8888/tcp \
                   -v /opt/vmclarity:/data \
                   --env-file /etc/vmclarity/config.env \
-                  tehsmash/vmclarity-backend:dc2d75a10e5583e97f516be26fcdbb484f98d5c3 run --log-level info
+                  idanfrim/vmclarity-backend:new-tag run --log-level info

                 [Install]
                 WantedBy=multi-user.target
```

# Go to AWS -> Cloudformation and create a stack.

* Ensure you have an SSH key pair uploaded to AWS Ec2
* Go to CloudFormation -> Create Stack -> From Template.
* Upload the modified VMClarity.cfn
* Follow the wizard through to the end
* Wait for install to complete

# SSH to the VMClarity server

* Get the public IP address of VMClarity backend from the CloudFormation stack's Output Tab
  ```
  ssh -i <your ssh key pair> ubuntu@<ip address>
  ```
* Check the VMClarity Logs
  ```
  sudo journalctl -u vmclarity
  ```

# Create Scan Config

* Copy the scanConfig.json into the ubuntu user's home directory
  ```
  scp -i <your ssh key pair> scanConfig.json ubuntu@<ip address>:~/scanConfig.json
  ```

* While ssh'd into the VMClarity server run, update `scanConfig.json` time and post to server
  ```
  curl -X POST http://localhost:8888/api/scanConfigs -H 'Content-Type: application/json' -d @scanConfig.json
  ```

* Watch the VMClarity logs again
  ```
  sudo journalctl -u vmclarity -f
  ```

# Debugging scanner VM

* The default security group doesn't allow inbound ssh communication, need to update the group after creation.
* Run `cat /var/log/cloud-init-output.log` inside the scanner vm to see logs from the cloud init process.
* Validate that there is data to scan in `/mnt/snapshot` since mount error are not shown in the cloud init logs.
* Run `sudo journalctl -u vmclarity-scanner -f` inside the scanner vm to see the service logs
  * The cloud init process takes some time, so be patient when waiting for logs from the scanner service.