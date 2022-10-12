#!/bin/bash

echo "####################################################################"
echo "## 11. dataDisk: Detach"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/vm/${CONN_CONFIG[$INDEX,$REGION]}-1?option=detachDataDisk -H 'Content-Type: application/json' -d \
		'{
			"dataDiskId": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'"
		}' | jq ''
