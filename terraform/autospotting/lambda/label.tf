module "label" {
  source  = "git::https://github.com/cloudposse/terraform-null-label.git?ref=0.5.4"
  context = "${var.label_context}"
}
