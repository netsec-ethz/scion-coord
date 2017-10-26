scionApp
    .controller('userCtrl', ['$scope', '$rootScope', 'userService', '$location', '$window', '$http',
        function ($scope, $rootScope, userService, $location, $window, $http) {

            $scope.error = "";
            $scope.message = "";

            $scope.userPageData = function () {

                userService.userPageData().then(
                    function (data) {
                        console.log(data);
                        $rootScope.user = data["User"];
                        $scope.vmInfo = data["VMInfo"];
                        $scope.buttonConfig = data["UIButtons"];
                        $scope.user.isNotVPN = false;
                    },
                    function (response) {
                        console.log(response);
                        if (response.status === 401 || response.status === 403) {
                            $location.path('/login');
                        }
                    });
            };

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

            let downloadlink = function (user) {
                return ('/api/as/downloads?filename=' + user["Email"] + '.tar.gz');
            };

            $scope.generateSCIONLabVM = function (user) {
                $scope.error = "";
                $scope.message = "";

                userService.generateSCIONLabVM(user).then(
                    function (data) {
                        console.log(data);
                        window.location.assign(downloadlink(user));
                        $scope.message = data;
                        $scope.userPageData();
                    },
                    function (response) {
                        console.log(response);
                        $scope.error = response.data;
                    });
            };

            $scope.downloadSCIONLabVM = function (user) {
                $scope.error = "";
                $scope.message = "";

                window.location.assign(downloadlink(user));
            };

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
        }
    ]);
