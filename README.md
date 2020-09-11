# Realizing_Microservices_HPC
Realizing Microservices and High Performance Computing

Project Logistics:
Mentors: Dan Lambright email: dlambrig@gmail.com
Will the project be open source: yes
 
Preferred Past Experience:
Kubernetes: Valuable
Go: Valuable 
Open Source Familiarity: Valuable 

Project Overview:
Background: 

This project is a continuation of work started in last year’s cloud computing course, which was presented at Linux Vault 2019. 
 
Lustre is a distributed file system used in high performance computing (HPC). Like other open source file systems (e.g. Ceph), it can run in Kubernetes, which extends it to the microservices world. This simplifies its management, as shown by students in the 2019 cloud computing course. Those students built the foundational infrastructure to package and start Lustre using “KubeVirt” (necessary to run virtual machines in containers- for Lustre’s kernel drivers). However, they did not automate the maintenance of Luster within Kubernetes. 

In a microservices environment, certain operations should be seamless:
When an instance of Lustre crashes, a new one should be created
It should be easy to auto-scale the number of instances (up or down)
Upgrading to new Luster code should be simple
It should be manageable via a dashboard

In short, Lustre should be a well-behaved microservice.
 
The students in the 2019 course showed Lustre had little performance degradation running in Kubevirt in containers. But they only experimented with a few nodes, and also did not have time to experiment with tools common in high performance computing, such as RDMA. This semester (time/resources permitting) the team will experiment with RDMA in Kubernetes. We will use the “freeflow” scheme developed at Microsoft. It removes the overhead of the “overlay network” used by containers. 
 
Project specifics: In this project you will write Go code to create “operators.” These are custom functions that are invoked when certain procedures are called in Kubernetes. Your code will be called, e.g. when a request is made to increase or decrease the number of nodes. 

You will run Microsoft’s freeflow overlay network and experiment with RDMA. The Microsoft work is open source. This will reduce the overhead over the “overlay network”. This will eliminate one more barrier to running high performance computing workloads in a microservices environment. 

More information:
2019 github site
2019 presentation at Vault conference
Microsoft “Freeflow”
Microsoft HPC container networking


Some Technologies you will learn/use:
Go programming
microservices
high performance computing problems
Virtual networking / network overlay
