# Update and install the cloud formation

## Build the containers and publish them to your docker hub

```
VMCLARITY_TOOLS_BASE=<your vmclarity tools base image> DOCKER_REGISTRY=<your docker hub> make push-docker
```

## Update installation/aws/VMClarity.cfn

Update the cloud formation with the pushed docker images, for example:

```
@@ -123,7 +123,7 @@ Resources:
                     DATABASE_DRIVER=LOCAL
                     BACKEND_REST_ADDRESS=__BACKEND_REST_ADDRESS__
                     BACKEND_REST_PORT=8888
-                    SCANNER_CONTAINER_IMAGE=tehsmash/vmclarity-cli:dc2d75a10e5583e97f516be26fcdbb484f98d5c3
+                    SCANNER_CONTAINER_IMAGE=tehsmash/vmclarity-cli:9bba94334c1de1aeed63ed12de3784d561fc4f1b
                   - JobImageID: !FindInMap
                       - AWSRegionArch2AMI
                       - !Ref "AWS::Region"
@@ -145,13 +145,13 @@ Resources:
                 ExecStartPre=-/usr/bin/docker stop %n
                 ExecStartPre=-/usr/bin/docker rm %n
                 ExecStartPre=/usr/bin/mkdir -p /opt/vmclarity
-                ExecStartPre=/usr/bin/docker pull tehsmash/vmclarity-backend:dc2d75a10e5583e97f516be26fcdbb484f98d5c3
+                ExecStartPre=/usr/bin/docker pull tehsmash/vmclarity-backend:9bba94334c1de1aeed63ed12de3784d561fc4f1b
                 ExecStart=/usr/bin/docker run \
                   --rm --name %n \
                   -p 0.0.0.0:8888:8888/tcp \
                   -v /opt/vmclarity:/data \
                   --env-file /etc/vmclarity/config.env \
-                  tehsmash/vmclarity-backend:dc2d75a10e5583e97f516be26fcdbb484f98d5c3 run --log-level info
+                  tehsmash/vmclarity-backend:9bba94334c1de1aeed63ed12de3784d561fc4f1b run --log-level info

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

* Get the IP address from the CloudFormation stack's Output Tab
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

* While ssh'd into the VMClarity server run
  ```
  curl -X POST http://localhost:8888/api/scanConfigs -H 'Content-Type: application/json' -d @scanConfig.json
  ```

* Watch the VMClarity logs again
  ```
  sudo journalctl -u vmclarity -f
  ```
