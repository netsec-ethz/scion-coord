angular.module('scionApp')
    .controller('registerCtrl', ['$scope', 'registerService', '$interval', '$location',
        function($scope, registerService, $interval, $location) {

            $scope.error = "";
            $scope.message = "";
            $scope.user = {};

            // refresh the list of processes
            $scope.register = function(user) {

                registerService.register(user).then(
                    function(data) {
                        $scope.message = "Registration completed successfully. We sent you an email to your inbox with a link to verify your account.";
                        $scope.error = ""
                        $scope.user = {};

                    },
                    function(response) {
                        $scope.error = response.data;
                        $scope.message = ""
                        console.log(response);
                    });
            };

        }
    ]);