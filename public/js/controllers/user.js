scionApp
    .controller('userCtrl', ['$scope', '$rootScope', 'userService', '$location', '$window', '$http', '$timeout',
        function ($scope, $rootScope, userService, $location, $window, $http, $timeout) {

            $scope.error1 = "";
            $scope.message1 = "";
            $scope.error2 = "";
            $scope.message2 = "";

            $scope.userPageData = function () {

                userService.userPageData().then(
                    function (data) {
                        console.log(data);
                        $rootScope.user = data["User"];
                        $scope.maxASes = data["MaxASes"];
                        $scope.aps = data["APs"];
                        $scope.asInfos = data["ASInfos"];
                        if ($scope.currentIndex === undefined) {
                            if ($scope.asInfos.length > 0)
                                $scope.asInfo = $scope.asInfos[0];
                        }
                        else
                            $scope.asInfo = $scope.asInfos[$scope.currentIndex];
                        $scope.grafanaLink = data["GrafanaLink"]
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
            };

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
            };

            $scope.buildImage = function (imageName, asInfo) {
                $scope.error2 = "";
                $scope.message2 = "";

                userService.startBuildJob(imageName, asInfo.ASID).then(
                    function (data) {
                        console.log(data);
                        $scope.message2 = data;

                        $scope.userImagesData();
                    },
                    function (response) {
                        console.log(response.data);
                        $scope.error2 = response.data;
                    });

                console.log("Creating image for: "+imageName)
            };

            $scope.generateSCIONLabAS = function () {
                setCurrentIndex();
                userService.generateSCIONLabAS().then(
                    function (data) {
                        console.log(data);
                        $scope.userPageData();
                        $scope.message1 = data;
                    },
                    function (response) {
                        console.log(response);
                        $scope.error1 = response.data;
                    });
            };

            $scope.submitForm = function (action, user, asInfo) {
                setCurrentIndex();
                switch (action) {
                    case "update":
                        if (!asInfo.IsVPN && !$scope.scionLabASForm.IP.$valid) {
                            $scope.error2 = "Please enter a correct public IP address.";
                        } else if (!$scope.scionLabASForm.Port.$valid) {
                            $scope.error2 = "Please enter a correct port in the range 1024-65535.";
                        } else if (!$scope.scionLabASForm.AP.$valid) {
                            $scope.error2 = "Please select an Attachment Point.";
                        } else {
                            $scope.configureSCIONLabAS(user, asInfo);
                        }
                        break;
                    case "download":
                        $scope.downloadSCIONLabAS(asInfo);
                        break;
                    case "remove":
                        $scope.removeSCIONLabAS(asInfo);
                        break;
                }
            };

            let setCurrentIndex = function () {
                $scope.currentIndex = $scope.asInfos.length > 0 ?
                    $scope.asInfos.indexOf($scope.asInfo) :
                    undefined;
            };

            let downloadlink = function (asID) {
                return ('/api/as/downloadTarball/' + asID);
            };

            $scope.downloadImage= function(userImage) {
                $scope.error2 = "";
                $scope.message2 = "";

                window.location.assign(userImage.download_link);
            };

            $scope.configureSCIONLabAS = function (user, asInfo) {
                $scope.error2 = "";
                $scope.message2 = "";

                userService.configureSCIONLabAS(user, asInfo).then(
                    function (data) {
                        console.log(data);
                        window.location.assign(downloadlink(asInfo.ASID));
                        $scope.message2 = data;
                        $scope.userPageData();
                    },
                    function (response) {
                        console.log(response);
                        $scope.error2 = response.data;
                    });
            };

            $scope.disableVPN = function (ap) {
                if (!$scope.aps[ap]["HasVPN"]) {
                    $scope.asInfo["IsVPN"] = false;
                }
            };

            $scope.downloadSCIONLabAS = function (asInfo) {
                $scope.error2 = "";
                $scope.message2 = "";

                window.location.assign(downloadlink(asInfo.ASID));
            };

            $scope.removeSCIONLabAS = function (asInfo) {
                $scope.error2 = "";
                $scope.message2 = "";

                userService.removeSCIONLabAS(asInfo.ASID).then(
                    function (data) {
                        console.log(data);
                        $scope.message2 = data;
                        $scope.userPageData();
                    },
                    function (response) {
                        console.log(response);
                        $scope.error2 = response.data;
                    });
            };

            $scope.dismissSuccess = function (i) {
                switch (i) {
                    case 1:
                        $scope.message1 = "";
                        break;
                    case 2:
                        $scope.message2 = "";
                }
            };

            $scope.dismissError = function (i) {
                switch (i) {
                    case 1:
                        $scope.error1 = "";
                        break;
                    case 2:
                        $scope.error2 = "";
                }
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
            // Watch for build images
            $scope.availableImagesData();
        }
    ]);
