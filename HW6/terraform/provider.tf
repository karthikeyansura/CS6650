# Specify where to find the AWS & Docker providers
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.7.0"
    }
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
  }
}

# Configure AWS provider
provider "aws" {
  region = var.aws_region
}

# Fetch ECR auth token for Docker provider login
data "aws_ecr_authorization_token" "registry" {}

# Docker provider authenticates against ECR to build & push images
provider "docker" {
  registry_auth {
    address  = data.aws_ecr_authorization_token.registry.proxy_endpoint
    username = data.aws_ecr_authorization_token.registry.user_name
    password = data.aws_ecr_authorization_token.registry.password
  }
}