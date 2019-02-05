module "label" {
  source      = "git::https://github.com/cloudposse/terraform-null-label.git?ref=master"
  context     = "${var.label_context}"
  namespace   = "${var.label_namespace}"
  environment = "${var.label_environment}"
  stage       = "${var.label_stage}"
  name        = "${var.label_name}"
  attributes  = ["${var.label_attributes}"]
  tags        = "${var.label_tags}"
  delimiter   = "${var.label_delimiter}"
}
