# MQTT ioMessage Broker


```yaml

apiVersion: datasance.com/v3
kind: Application
metadata:
  name: test
spec:
  microservices:
    - name: mqtt-iomessage
      agent:
        name: edge-1
      images:
        registry: 1
        catalogItemId: null
        x86: ghcr.io/datasance/mqtt2iomessage:latest
        arm: ghcr.io/datasance/mqtt2iomessage:latest
      container:
        rootHostAccess: false
        cdiDevices: []
        ports: []
        volumes: []
        env: []
        extraHosts: []
        commands: []
      msRoutes:
        pubTags:
          - mqtt
      config:
        mqttHost: iofog
        mqttPort: 1883
        token: 
        topics:
          - mainTopic: test
            subtopics:
              - test
              - test2
            infoType: app/json
            infoFormat: app/json

```
