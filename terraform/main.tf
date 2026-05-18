terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
  }
}

provider "docker" {}

# Создаём два контейнера, которые будут имитировать виртуальные машины
resource "docker_container" "vm1" {
  name  = "shopflow-terraform-vm1"
  image = "alpine:latest"
  command = ["sleep", "infinity"]
  restart = "always"
}

resource "docker_container" "vm2" {
  name  = "shopflow-terraform-vm2"
  image = "alpine:latest"
  command = ["sleep", "infinity"]
  restart = "always"
}

output "vm1_id" {
  value = docker_container.vm1.id
}

output "vm2_id" {
  value = docker_container.vm2.id
}

output "vm1_name" {
  value = docker_container.vm1.name
}

output "vm2_name" {
  value = docker_container.vm2.name
}