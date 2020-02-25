# Vearch Compile and Deploy

## Check config

#### modify config 

   1. cd config 
   2. modify config.toml.example file
   3. such as deploy two master pod,get master pod ip
   4. modify address to new ip      
      
##  Docker install

####  delete old docker 
* sudo yum remove -y docker \
                  docker-client \
                  docker-client-latest \
                  docker-common \
                  docker-latest \
                  docker-latest-logrotate \
                  docker-logrotate \
                  docker-selinux \
                  docker-engine-selinux \
                  docker-engine

#### install docker
* sudo yum install -y yum-utils device-mapper-persistent-data lvm2
* sudo yum-config-manager --add-repo  https://download.docker.com/linux/centos/docker-ce.repo
* sudo yum install docker -y

#### start docker
* modify /etc/sysconfig/docker file
* OPTIONS='--selinux-enabled=false --log-driver=json-file --signature-verification=false'
#### modify systemd start param
* mv /etc/systemd/system/docker.service.d/execstart.conf /etc/systemd/system/docker.service.d/execstart.conf.cp
* systemctl daemon-reload
* systemctl enable docker && systemctl start docker
      

## Make Docker Image
* go to $vearch/cloud dir
* you can run `./run_docker.sh`  
* docker push image center,such as  docker push xx.xx.local/ai/vearch:0.3 
       
## Deploy
* new master.yaml
      
```
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: vearchmaster
  namespace: vearch
spec:
  replicas: 2
  selector:
    matchLabels:
      app: vearchmaster
  revisionHistoryLimit: 3
  template:
    metadata:
      labels:
        app: vearchmaster
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node.bcc.XX.com/dedicated
                operator: In
                values:
                - face
      terminationGracePeriodSeconds: 0
      tolerations:
      - effect: NoSchedule
        key: nvidia.com/gpu
        operator: Exists
      - effect: NoSchedule
        key: node.bcc.XX.com/dedicated
        operator: Equal
        value: vearch
      containers:
      - name: vearchmaster
        command: ["./bin/start.sh master"]
        image: xx.xx.local/ai/vearch:0.3
        resources:
          limits:
            cpu: 16
            memory: 16Gi
          requests:
            cpu: 16
            memory: 16Gi
        env:
        - name: TZ
          value: Asia/Shanghai

```

