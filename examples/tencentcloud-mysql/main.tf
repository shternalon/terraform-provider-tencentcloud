resource "tencentcloud_mysql_instance" "main" {
  mem_size          = 1000
  volume_size       = 50
  instance_name     = "testAccMysql"
  engine_version    = "5.7"
  root_password     = "test1234"
  availability_zone = var.availability_zone
  internet_service  = 1
  slave_sync_mode   = 1
  intranet_port     = 3360

  tags = {
    purpose = "for test"
  }

  parameters = {
    max_connections = "1000"
  }
}

resource "tencentcloud_mysql_readonly_instance" "readonly" {
  master_instance_id = tencentcloud_mysql_instance.main.id
  mem_size           = 1000
  volume_size        = 50
  instance_name      = "testAccMysql_readonly"
  intranet_port      = 3360

  tags = {
    purpose = "for test"
  }
}

data "tencentcloud_mysql_parameter_list" "mysql" {
  mysql_id = tencentcloud_mysql_instance.main.id
}

resource "tencentcloud_mysql_account" "mysql_account" {
  mysql_id    = tencentcloud_mysql_instance.main.id
  name        = "test"
  password    = "test1234"
  description = "for test"
}

resource "tencentcloud_mysql_privilege" "privilege" {
  mysql_id     = tencentcloud_mysql_instance.main.id
  account_name = tencentcloud_mysql_account.mysql_account.name
  global       = ["TRIGGER"]
  database {
    privileges    = ["SELECT", "INSERT", "UPDATE", "DELETE", "CREATE"]
    database_name = "sys"
  }
  database {
    privileges    = ["SELECT"]
    database_name = "performance_schema"
  }

  table {
    privileges    = ["SELECT", "INSERT", "UPDATE", "DELETE", "CREATE"]
    database_name = "mysql"
    table_name    = "slow_log"
  }

  table {
    privileges    = ["SELECT", "INSERT", "UPDATE"]
    database_name = "mysql"
    table_name    = "user"
  }

  column {
    privileges    = ["SELECT", "INSERT", "UPDATE", "REFERENCES"]
    database_name = "mysql"
    table_name    = "user"
    column_name   = "host"
  }
}

resource "tencentcloud_mysql_backup_policy" "mysql_backup_policy" {
  mysql_id         = tencentcloud_mysql_instance.main.id
  retention_period = 56
  backup_model     = "physical"
  backup_time      = "10:00-14:00"
}


