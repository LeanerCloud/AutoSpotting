// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"encoding/base64"
	"strings"
)

// Beanstalk UserData wrappers for CloudFormation Helper scripts
// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cfn-helper-scripts-reference.html
//
// `cfn-init`, `cfn-get-metadata` and `cfn-signal` are wrapped by adding the
// instance role to the original code as `--role instance-role`
// `cfn-hup` does not accept a `--role` param so we write the role into the config file
// /etc/cfn/cfn-hup.conf
var beanstalkUserDataCFNWrappers = `---- modify CloudFormation helpers ----
# Modify cfn-init to use --role by default
echo -e '#!/bin/bash\nINROLE=$(curl -s 169.254.169.254/latest/meta-data/iam/security-credentials/)\n/opt/aws/bin/cfn-init-2 --role $INROLE "$@" \nexit $?' > /opt/aws/bin/cfn-init.tmp
mv /opt/aws/bin/cfn-init /opt/aws/bin/cfn-init-2
mv /opt/aws/bin/cfn-init.tmp /opt/aws/bin/cfn-init
chmod +x /opt/aws/bin/cfn-init

# Modify cfn-get-metadata to use --role by default
echo -e '#!/bin/bash\nINROLE=$(curl -s 169.254.169.254/latest/meta-data/iam/security-credentials/)\n/opt/aws/bin/cfn-get-metadata-2 --role $INROLE "$@" \nexit $?' > /opt/aws/bin/cfn-get-metadata.tmp
mv /opt/aws/bin/cfn-get-metadata /opt/aws/bin/cfn-get-metadata-2
mv /opt/aws/bin/cfn-get-metadata.tmp /opt/aws/bin/cfn-get-metadata
chmod +x /opt/aws/bin/cfn-get-metadata

# Modify cfn-signal to use --role by default
echo -e '#!/bin/bash\nINROLE=$(curl -s 169.254.169.254/latest/meta-data/iam/security-credentials/)\n/opt/aws/bin/cfn-signal-2 --role $INROLE "$@" \nexit $?' > /opt/aws/bin/cfn-signal.tmp
mv /opt/aws/bin/cfn-signal /opt/aws/bin/cfn-signal-2
mv /opt/aws/bin/cfn-signal.tmp /opt/aws/bin/cfn-signal
chmod +x /opt/aws/bin/cfn-signal

# Modify cfn-hup to use --role by default
echo -e '#!/bin/bash\nprintf "role=$(curl -s 169.254.169.254/latest/meta-data/iam/security-credentials/)" >> /etc/cfn/cfn-hup.conf\n/opt/aws/bin/cfn-hup-2 "$@" \nexit $?' > /opt/aws/bin/cfn-hup.tmp
mv /opt/aws/bin/cfn-hup /opt/aws/bin/cfn-hup-2
mv /opt/aws/bin/cfn-hup.tmp /opt/aws/bin/cfn-hup
chmod +x /opt/aws/bin/cfn-hup
---- modify CloudFormation helpers ----

`

func decodeUserData(userData *string) *string {
	// UserData is sometimes encoded as base64 ; decoded it if needed
	decodedUserData, err := base64.StdEncoding.DecodeString(*userData)

	if err != nil {
		// This is not Base64-encoded, return the original string
		return userData
	}

	// This was Base64-encoded, return the decoded string
	decodedUserDataString := string(decodedUserData)
	return &decodedUserDataString
}

func encodeUserData(userData *string) *string {
	// Encode UserData string to base64
	encodedUserData := base64.StdEncoding.EncodeToString([]byte(*userData))

	return &encodedUserData
}

func getPatchedUserDataForBeanstalk(userData *string) *string {
	// Decode the UserData
	decodedUserData := decodeUserData(userData)

	// Patch the UserData if possible
	if strings.Contains(*decodedUserData, "ebbootstrap") {
		// Force set the role for calling CloudFormation helpers to be the instance role
		// The UserData created by Beanstalk is encoded as a Mime Multi Part Archive
		// with Cloud Init User-Data format (https://cloudinit.readthedocs.io/en/latest/topics/format.html)
		// We can't simply append our extra code to it, we need to add it to the correct mime part
		// Hence, we replace the first `#!/bin/bash` with our wrapper
		patchedUserData := strings.Replace(*decodedUserData, "#!/bin/bash\n", "#!/bin/bash\n"+beanstalkUserDataCFNWrappers, 1)
		return encodeUserData(&patchedUserData)
	}

	return userData
}
