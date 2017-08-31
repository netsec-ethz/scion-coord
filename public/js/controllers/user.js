angular.module('scionApp')
    .controller('userCtrl', ['$scope', 'userService', '$location', '$window', '$http',
        function ($scope, userService, $location, $window, $http) {

            $scope.error = "";
            $scope.message = "";

            $scope.me = function () {

                userService.me().then(
                    function (data) {
                        console.log(data);
                        $scope.user = data["User"];
                        $scope.vmInfo = data["VMInfo"]
                        $scope.buttonConfig = data["ButtonConfig"]
                    },
                    function (response) {
                        console.log(response);
                        //$location.path('/');
                    });
            };

            $scope.generateSCIONLabVM = function (user) {
                $scope.error = "";
                $scope.message = "";

                userService.generateSCIONLabVM(user).then(
                    function (data) {
                        console.log(data);
                        window.location.assign('/api/as/downloads?filename=' + data["filename"]);
                        $scope.message = data["message"];
                    },
                    function (response) {
                        console.log(response);
                        $scope.error = response.data;
                    });
            };

            $scope.downloadSCIONLabVM = function (user) {
                $scope.error = "";
                $scope.message = "";

                window.location.assign('/api/as/downloads?filename=' + user["Email"] + '.tar.gz');
            };

            $scope.removeSCIONLabVM = function (user) {
                $scope.error = "";
                $scope.message = "";

                userService.removeSCIONLabVM(user).then(
                    function (data) {
                        console.log(data);
                        $scope.message = data;
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
            angular.element($window).bind('resize', function(){
                $scope.$apply();
            });

            // refresh the data when the controller is loaded
            $scope.me();
        }
    ]);