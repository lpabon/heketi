#!/bin/sh

vagrant up --no-provision $@
vagrant provision
vagrant ssh client --command "ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook -i hosts.conf openshift-ansible/playbooks/byo/config.yml"
