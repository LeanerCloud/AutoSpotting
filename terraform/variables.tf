# Autospotting configuration
variable "asg_min_on_demand_number" {
  description = "Minimum on demand number for all ASG enabled"
  default = "0"
}

variable "asg_min_on_demand_percentage" {
  description = "Minimum on demand percentage for all ASG enabled"
  default = "0.0"
}

variable "asg_regions_enabled" {
  description = "Regions in which autospotting is enabled"
  default = ""
}

# Lambda configuration
variable "lambda_zipname" {
  description = "Name of the archive"
  default = "../build/s3/dv/lambda.zip"
}

variable "lambda_runtime" {
  description = "Environment the lambda function runs in"
  default = "python2.7"
}

variable "lambda_memory_size" {
  description = "Memory size allocated to the lambda run"
  default = 256
}

variable "lambda_timeout" {
  description = "Timeout after which the lambda timeout"
  default = 300
}

variable "lambda_run_frequency" {
  description = "How frequent should lambda run"
  default = "rate(5 minutes)"
}
