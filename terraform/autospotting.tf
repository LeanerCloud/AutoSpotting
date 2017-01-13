module "autospotting" {
  source = "./autospotting"

  autospotting_min_on_demand_number = "${var.asg_min_on_demand_number}"
  autospotting_min_on_demand_percentage = "${var.asg_min_on_demand_percentage}"
  autospotting_regions_enabled = "${var.asg_regions_enabled}"

  lambda_zipname = "${var.lambda_zipname}"
  lambda_runtime = "${var.lambda_runtime}"
  lambda_memory_size = "${var.lambda_memory_size}"
  lambda_timeout = "${var.lambda_timeout}"
  lambda_run_frequency = "${var.lambda_run_frequency}"
}
