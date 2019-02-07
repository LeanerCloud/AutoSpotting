module "label" {
  source      = "git::https://github.com/cloudposse/terraform-null-label.git?ref=0.5.4"
  context     = "${var.label_context}"
  namespace   = "${var.label_namespace}"
  environment = "${var.label_environment}"
  stage       = "${var.label_stage}"
  name        = "${var.label_name}"
  attributes  = ["${var.label_attributes}"]
  tags        = "${var.label_tags}"
  delimiter   = "${var.label_delimiter}"
}

module "autospotting" {
  source = "./autospotting"

  autospotting_allowed_instance_types       = "${var.asg_allowed_instance_types}"
  autospotting_disallowed_instance_types    = "${var.asg_disallowed_instance_types}"
  autospotting_min_on_demand_number         = "${var.asg_min_on_demand_number}"
  autospotting_min_on_demand_percentage     = "${var.asg_min_on_demand_percentage}"
  autospotting_on_demand_price_multiplier   = "${var.asg_on_demand_price_multiplier}"
  autospotting_spot_price_buffer_percentage = "${var.asg_spot_price_buffer_percentage}"
  autospotting_spot_product_description     = "${var.asg_spot_product_description}"
  autospotting_bidding_policy               = "${var.asg_bidding_policy}"
  autospotting_regions_enabled              = "${var.asg_regions_enabled}"
  autospotting_tag_filters                  = "${var.asg_tag_filters}"
  autospotting_tag_filtering_mode           = "${var.asg_tag_filtering_mode}"

  lambda_zipname       = "${var.lambda_zipname}"
  lambda_s3_bucket     = "${var.lambda_s3_bucket}"
  lambda_s3_key        = "${var.lambda_s3_key}"
  lambda_runtime       = "${var.lambda_runtime}"
  lambda_memory_size   = "${var.lambda_memory_size}"
  lambda_timeout       = "${var.lambda_timeout}"
  lambda_run_frequency = "${var.lambda_run_frequency}"
  lambda_tags          = "${var.lambda_tags}"

  label_context     = "${module.label.context}"
  label_namespace   = "${module.label.namespace}"
  label_environment = "${module.label.environment}"
  label_stage       = "${module.label.stage}"
  label_name        = "${module.label.name}"
  label_attributes  = "${module.label.attributes}"
  label_tags        = "${module.label.tags}"
  label_delimiter   = "${module.label.delimiter}"
}
