<a id="top" name="top"></a>
[<img src="_asset/logo_powered-by-aws.png" alt="Powered by Amazon Web Services" align="right">][aws-home]
[<img src="_asset/logo_created-by-eawsy.png" alt="Created by eawsy" align="right">][eawsy-home]

# eawsy/aws-lambda-go-core

> Type definitions and helpers for AWS Lambda Go runtime.

[![Api][badge-api]][eawsy-api]
[![Status][badge-status]](#top)
[![License][badge-license]](LICENSE)
[![Help][badge-help]][eawsy-chat]
[![Social][badge-social]][eawsy-twitter]

[AWS Lambda][aws-lambda-home] lets you run code without provisioning or managing servers. With 
[eawsy/aws-lambda-go-shim][eawsy-runtime], you can author your Lambda function code in Go. This project provides type 
definitions and helpers to deal with AWS Lambda Go runtime. 

## Preview

> For step by step instructions on how to author your AWS Lambda function code in Go, see 
  [eawsy/aws-lambda-go-shim][eawsy-runtime].

```sh
go get -u -d github.com/eawsy/aws-lambda-go-core/...
```

```go
package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/eawsy/aws-lambda-go-core/service/lambda/runtime"
)

func Handle(evt json.RawMessage, ctx *runtime.Context) (interface{}, error) {
	log.Printf("Log stream name: %s\n", ctx.LogStreamName)
	log.Printf("Log group name: %s\n", ctx.LogGroupName)
	log.Printf("Request ID: %s\n", ctx.AWSRequestID)
	log.Printf("Mem. limits(MB): %d\n", ctx.MemoryLimitInMB)

	select {
	case <-time.After(1 * time.Second):
		log.Printf("Time remaining (MS): %d\n", ctx.RemainingTimeInMillis())
	}

	return nil, nil
}
```

[<img src="_asset/misc_arrow-up.png" align="right">](#top)
## About

[![eawsy](_asset/logo_eawsy.png)][eawsy-home]

This project is maintained and funded by Alsanium, SAS.

[We][eawsy-home] :heart: [AWS][aws-home] and open source software. See [our other projects][eawsy-github], or 
[hire us][eawsy-hire] to help you build modern applications on AWS.

[<img src="_asset/misc_arrow-up.png" align="right">](#top)
## Contact

We want to make it easy for you, users and contributers, to talk with us and connect with each others, to share ideas, 
solve problems and make help this project awesome. Here are the main channels we're running currently and we'd love to 
hear from you on them.

### Twitter 
  
[eawsyhq][eawsy-twitter] 

Follow and chat with us on Twitter. 

Share stories!

### Gitter 

[eawsy/bavardage][eawsy-chat]

This is for all of you. Users, developers and curious. You can find help, links, questions and answers from all the 
community including the core team.

Ask questions!

### GitHub

[pull requests][eawsy-pr] & [issues][eawsy-issues]

You are invited to contribute new features, fixes, or updates, large or small; we are always thrilled to receive pull 
requests, and do our best to process them as fast as we can.

Before you start to code, we recommend discussing your plans through the [eawsy/bavardage channel][eawsy-chat], 
especially for more ambitious contributions. This gives other contributors a chance to point you in the right direction, 
give you feedback on your design, and help you find out if someone else is working on the same thing.

Write code!

[<img src="_asset/misc_arrow-up.png" align="right">](#top)
## License

This product is licensed to you under the Apache License, Version 2.0 (the "License"); you may not use this product 
except in compliance with the License. See [LICENSE](LICENSE) and [NOTICE](NOTICE) for more information.

[<img src="_asset/misc_arrow-up.png" align="right">](#top)
## Trademark

Alsanium, eawsy, the "Created by eawsy" logo, and the "eawsy" logo are trademarks of Alsanium, SAS. or its affiliates in 
France and/or other countries.

Amazon Web Services, the "Powered by Amazon Web Services" logo, and AWS Lambda are trademarks of Amazon.com, Inc. or its 
affiliates in the United States and/or other countries.

[eawsy-home]: https://eawsy.com
[eawsy-github]: https://github.com/eawsy/
[eawsy-api]: https://godoc.org/github.com/eawsy/aws-lambda-go-core/service/lambda/runtime
[eawsy-chat]: https://gitter.im/eawsy/bavardage
[eawsy-twitter]: https://twitter.com/@eawsyhq
[eawsy-hire]: https://docs.google.com/forms/d/e/1FAIpQLSfPvn1Dgp95DXfvr3ClPHCNF5abi4D1grveT5btVyBHUk0nXw/viewform
[eawsy-pr]: https://github.com/eawsy/aws-lambda-go-core/issues?q=is:pr%20is:open
[eawsy-issues]: https://github.com/eawsy/aws-lambda-go-core/issues?q=is:issue%20is:open
[eawsy-runtime]: https://github.com/eawsy/aws-lambda-go-shim

[aws-home]: https://aws.amazon.com
[aws-lambda-home]: https://aws.amazon.com/lambda

[badge-api]: http://img.shields.io/badge/api-godoc-3F51B5.svg?style=flat-square
[badge-status]: http://img.shields.io/badge/status-stable-4CAF50.svg?style=flat-square
[badge-license]: http://img.shields.io/badge/license-apache-FF5722.svg?style=flat-square
[badge-help]: http://img.shields.io/badge/help-gitter-E91E63.svg?style=flat-square
[badge-social]: http://img.shields.io/badge/social-twitter-03A9F4.svg?style=flat-square
