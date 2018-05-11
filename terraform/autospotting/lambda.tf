module "aws_lambda_function" {
  source = "./lambda"

  lambda_zipname = "${var.lambda_zipname}"

  lambda_s3_bucket = "${var.lambda_s3_bucket}"
  lambda_s3_key    = "${var.lambda_s3_key}"

  lambda_role_arn    = "${aws_iam_role.autospotting_role.arn}"
  lambda_runtime     = "${var.lambda_runtime}"
  lambda_timeout     = "${var.lambda_timeout}"
  lambda_memory_size = "${var.lambda_memory_size}"
  lambda_tags        = "${var.lambda_tags}"

  autospotting_allowed_instance_types       = "${var.autospotting_allowed_instance_types}"
  autospotting_disallowed_instance_types    = "${var.autospotting_disallowed_instance_types}"
  autospotting_min_on_demand_number         = "${var.autospotting_min_on_demand_number}"
  autospotting_min_on_demand_percentage     = "${var.autospotting_min_on_demand_percentage}"
  autospotting_on_demand_price_multiplier   = "${var.autospotting_on_demand_price_multiplier}"
  autospotting_spot_price_buffer_percentage = "${var.autospotting_spot_price_buffer_percentage}"
  autospotting_spot_product_description     = "${var.autospotting_spot_product_description}"
  autospotting_bidding_policy               = "${var.autospotting_bidding_policy}"
  autospotting_regions_enabled              = "${var.autospotting_regions_enabled}"
  autospotting_tag_filters                  = "${var.autospotting_tag_filters}"
  autospotting_tag_filtering_mode           = "${var.autospotting_tag_filtering_mode}"
}

resource "aws_iam_role" "autospotting_role" {
  name                  = "autospotting"
  path                  = "/lambda/"
  assume_role_policy    = "${file("${path.module}/lambda-policy.json")}"
  force_detach_policies = true
}

resource "aws_iam_role_policy" "autospotting_policy" {
  name   = "policy_for_autospotting"
  role   = "${aws_iam_role.autospotting_role.id}"
  policy = "${file("${path.module}/autospotting-policy.json")}"
}

resource "aws_lambda_permission" "cloudwatch_events_permission" {
  statement_id  = "AllowExecutionFromCloudWatch"
  action        = "lambda:InvokeFunction"
  function_name = "${module.aws_lambda_function.function_name}"
  principal     = "events.amazonaws.com"
  source_arn    = "${aws_cloudwatch_event_rule.cloudwatch_frequency.arn}"
}

resource "aws_cloudwatch_event_target" "cloudwatch_target" {
  rule      = "${aws_cloudwatch_event_rule.cloudwatch_frequency.name}"
  target_id = "run_autospotting"
  arn       = "${module.aws_lambda_function.arn}"
}

resource "aws_cloudwatch_event_rule" "cloudwatch_frequency" {
  name                = "autospotting_frequency"
  schedule_expression = "${var.lambda_run_frequency}"
}

resource "aws_cloudwatch_log_group" "log_group_autospotting" {
  name              = "/aws/lambda/${module.aws_lambda_function.function_name}"
  retention_in_days = 7
}
