scionApp
    .controller('userCtrl', ['$scope', '$rootScope', 'userService', '$location', '$window', '$http', '$timeout',
        function ($scope, $rootScope, userService, $location, $window, $http, $timeout) {

            $scope.error = "";
            $scope.message = "";

            $scope.isVmReady=false

            $scope.userPageData = function () {

                userService.userPageData().then(
                    function (data) {
                        console.log(data);
                        $rootScope.user = data["User"];
                        $scope.vmInfo = data["VMInfo"];
                        $scope.buttonConfig = data["UIButtons"];
                        $scope.user.isNotVPN = false;
                        // We want to show image building options only if we have VM config ready
                        $scope.isVmReady=$scope.vmInfo.VMStatus==1;
                    },
                    function (response) {
                        console.log(response);
                        if (response.status === 401 || response.status === 403) {
                            $location.path('/login');
                        }
                    });
            };

            $scope.userImagesData = function(done) {
                userService.getUserBuildImages().then(
                    function (data) {
                        console.log("Received user images")
                        data.forEach(function(userImg){
                            userImg.displayName=$scope.imageNames[userImg.image]
                        })
                        console.log(data);
                        $scope.userImages=data;

                        if(done!=null){
                            done()
                        }
                    },
                    function (response) {
                        console.log(response);
                        if (response.status === 401 || response.status === 403) {
                            $location.path('/login');
                        }else{
                            if(done!=null){
                                done()
                            }
                        }
                    });  
            }

            $scope.availableImagesData = function() {
                userService.getAvailableImages().then(
                    function (data) {
                        console.log("Received available images")
                        console.log(data);
                        $scope.availableImages=data;
                        $scope.imageNames={}
                        
                        data.forEach(function(img){
                            console.log("Setting up: "+img.name+" "+img.display_name)
                            $scope.imageNames[img.name]=img.display_name    
                        });

                        (function poll() {
                            $scope.userImagesData(function(){
                                $timeout(poll, 15000);
                            })
                        })();
                    },
                    function (response) {
                        console.log(response);
                        if (response.status === 401 || response.status === 403) {
                            $location.path('/login');
                        }
                    });  
            }

            $scope.submitForm = function (action, user) {
                switch (action) {
                    case "update":
                        if (user.isNotVPN) {
                            $scope.scionLabVMForm.scionLabVMIP.$setValidity("required", user.scionLabVMIP != null);
                        }
                        if (user.isNotVPN && !$scope.scionLabVMForm.scionLabVMIP.$valid) {
                            $scope.error = "Please enter a correct public IP address.";
                        } else {
                            $scope.generateSCIONLabVM(user);
                        }
                        break;
                    case "download":
                        $scope.downloadSCIONLabVM(user);
                        break;
                    case "remove":
                        $scope.removeSCIONLabVM(user);
                        break;
                }
            };

            let downloadlink = function () {
                return ('/api/as/downloadTarball');
            };

            $scope.generateSCIONLabVM = function (user) {
                $scope.error = "";
                $scope.message = "";

                userService.generateSCIONLabVM(user).then(
                    function (data) {
                        console.log(data);
                        window.location.assign(downloadlink());
                        $scope.message = data;
                        $scope.userPageData();
                    },
                    function (response) {
                        console.log(response);
                        $scope.error = response.data;
                    });
            };

            $scope.buildImage = function (imageName) {
                $scope.error = "";
                $scope.message = "";

                userService.startBuildJob(imageName).then(
                    function (data) {
                        console.log(data);
                        $scope.message = data;

                        $scope.userImagesData();
                    },
                    function (response) {
                        console.log(response);
                        $scope.error = response.data;
                    });

                console.log("Creating image for: "+imageName)
            };

            $scope.downloadSCIONLabVM = function (user) {
                $scope.error = "";
                $scope.message = "";

                window.location.assign(downloadlink());
            };

            $scope.downloadImage= function(userImage) {
                $scope.error = "";
                $scope.message = "";
                
                window.location.assign(userImage.download_link);                  
            }

            $scope.removeSCIONLabVM = function (user) {
                $scope.error = "";
                $scope.message = "";

                userService.removeSCIONLabVM(user).then(
                    function (data) {
                        console.log(data);
                        $scope.message = data;
                        $scope.userPageData();
                    },
                    function (response) {
                        console.log(response);
                        $scope.error = response.data;
                    });
            };

            $scope.dismissSuccess = function () {
                $scope.message = "";
            };

            $scope.dismissError = function () {
                $scope.error = "";
            };

            $scope.$watch(
                function () {
                    return $window.innerWidth;
                },
                function (value) {
                    $scope.isSmall = $window.innerWidth < 992;
                },
                true
            );
            angular.element($window).bind('resize', function () {
                $scope.$apply();
            });

            // refresh the data when the controller is loaded
            $scope.userPageData();
            // For building images
            $scope.availableImagesData();
        }
    ]);
