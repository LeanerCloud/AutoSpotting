module "label" {
  source  = "git::https://github.com/cloudposse/terraform-null-label.git?ref=master"
  context = "${var.label_context}"
}
