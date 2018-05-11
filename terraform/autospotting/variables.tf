# Autospotting configuration
variable "autospotting_allowed_instance_types" {
  description = <<EOF
Comma separated list of allowed instance types for spot requests,
in case you want to exclude specific types (also support globs).

Example: 't2.*,m4.large'

Using the 'current' magic value will only allow the same type as the
on-demand instances set in the group's launch configuration.
EOF

  default = ""
}

variable "autospotting_disallowed_instance_types" {
  description = <<EOF
Comma separated list of disallowed instance types for spot requests,
in case you want to exclude specific types (also support globs).

Example: 't2.*,m4.large'
EOF

  default = ""
}

variable "autospotting_min_on_demand_number" {
  description = "Minimum on-demand instances to keep in absolute value"
}

variable "autospotting_min_on_demand_percentage" {
  description = "Minimum on-demand instances to keep in percentage"
}

variable "autospotting_on_demand_price_multiplier" {
  description = "Multiplier for the on-demand price"
}

variable "autospotting_spot_price_buffer_percentage" {
  description = "Percentage above the current spot price to place the bid"
}

variable "autospotting_bidding_policy" {
  description = "Bidding policy for the spot bid"
}

variable "autospotting_regions_enabled" {
  description = "Regions that autospotting is watching"
}

variable "autospotting_tag_filters" {
  description = <<EOF
  Tags to filter which ASGs autospotting considers. By default, ASGs tagged
  with spot-enbled=true will be operated on. In opt-out mode it operates on
  all groups except those tagged with spot-enbled=false.

  You can set this to many tags, for example:
  spot-enabled=true,Environment=dev,Team=vision"
  EOF
}

variable "autospotting_tag_filtering_mode" {
  description = <<EOF
  Controls the tag-based ASG filter. Supported values: 'opt-in' or 'opt-out'.
  Defaults to opt-in mode, in which it only acts against the tagged groups. In
  opt-out mode it works against all groups except for the tagged ones.
  Use the opt-out mode carefully!
  EOF

  default = "opt-in"
}

variable "autospotting_spot_product_description" {
  description = "The Spot Product or operating system to use when looking up spot price history in the market. Valid choices: Linux/UNIX | SUSE Linux | Windows | Linux/UNIX (Amazon VPC) | SUSE Linux (Amazon VPC) | Windows (Amazon VPC)"
}

# Lambda configuration
variable "lambda_zipname" {
  description = "Name of the archive"
  default     = ""
}

variable "lambda_s3_bucket" {
  description = "Bucket which the archive is stored in"
  default     = ""
}

variable "lambda_s3_key" {
  description = "Key in S3 under which the archive is stored"
  default     = ""
}

variable "lambda_runtime" {
  description = "Environment the lambda function runs in"
}

variable "lambda_memory_size" {
  description = "Memory size allocated to the lambda run"
}

variable "lambda_timeout" {
  description = "Timeout after which the lambda timeout"
}

variable "lambda_run_frequency" {
  description = "How frequent should lambda run"
}

variable "lambda_tags" {
  description = "Tags to be applied to the Lambda function"
  type        = "map"
}
