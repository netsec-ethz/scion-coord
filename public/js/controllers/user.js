angular.module('scionApp')
    .controller('userCtrl', ['$scope', 'userService', '$location', '$window', '$http',
        function($scope, userService, $location, $window, $http) {

            $scope.error = "";
            $scope.message = "";

            $scope.me = function() {

                userService.me().then(
                    function(data) {
                        console.log(data);
                        $scope.user = data;
                    },
                    function(response) {
                        console.log(response);
                        //$location.path('/');
                    });
            };

            $scope.generateSCIONLabVM = function(user) {
                $scope.error = "";
                $scope.message = "";

                userService.generateSCIONLabVM(user).then(
                    function(data) {
                        console.log(data);
                        window.location.assign('/api/as/downloads?filename=' + data);
                        $scope.message = "Your VM will be activated within a few minutes.";
                    },
                    function(response) {
                        console.log(response);
                        $scope.error = response.data;
                    });
            };

            $scope.removeSCIONLabVM = function(user) {
                $scope.error = "";
                $scope.message = "";

                userService.removeSCIONLabVM(user).then(
                    function(data) {
                        console.log(data);
                        $scope.message = data;
                    },
                    function(response) {
                        console.log(response);
                        $scope.error = response.data;
                    });
            };

            // refresh the data when the controller is loaded
            $scope.me();
     }
    ]);
