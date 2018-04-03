output "function_name" {
  value = "${element(concat(aws_lambda_function.autospotting.*.function_name, aws_lambda_function.autospotting_from_s3.*.function_name), 0)}"
}

output "arn" {
  value = "${element(concat(aws_lambda_function.autospotting.*.arn, aws_lambda_function.autospotting_from_s3.*.arn), 0)}"
}
