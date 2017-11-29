# Autospotting configuration
variable "autospotting_min_on_demand_number" {
  description = "Minimum on-demand instances to keep in absolute value"
}

variable "autospotting_min_on_demand_percentage" {
  description = "Minimum on-demand instances to keep in percentage"
}

variable "autospotting_on_demand_price_multiplier" {
  description = "Multiplier for the on-demand price"
}

variable "autospotting_regions_enabled" {
  description = "Regions that autospotting is watching"
}

# Lambda configuration
variable "lambda_zipname" {
  description = "Name of the archive"
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
