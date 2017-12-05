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
                        $scope.asInfo = data["ASInfo"];
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
                            $scope.scionLabASForm.scionLabASIP.$setValidity("required", user.scionLabASIP != null);
                        }
                        if (user.isNotVPN && !$scope.scionLabASForm.scionLabASIP.$valid) {
                            $scope.error = "Please enter a correct public IP address.";
                        } else {
                            $scope.generateSCIONLabAS(user);
                        }
                        break;
                    case "download":
                        $scope.downloadSCIONLabAS(user);
                        break;
                    case "remove":
                        $scope.removeSCIONLabAS(user);
                        break;
                }
            };

            let downloadlink = function () {
                return ('/api/as/downloadTarball');
            };

            $scope.generateSCIONLabAS = function (user) {
                $scope.error = "";
                $scope.message = "";

                userService.generateSCIONLabAS(user).then(
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

            $scope.downloadSCIONLabAS = function (user) {
                $scope.error = "";
                $scope.message = "";

                window.location.assign(downloadlink());
            };

            $scope.removeSCIONLabAS = function (user) {
                $scope.error = "";
                $scope.message = "";

                userService.removeSCIONLabAS(user).then(
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
