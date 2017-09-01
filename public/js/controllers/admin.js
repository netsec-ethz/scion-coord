angular.module('scionApp')
    .controller('adminCtrl', ['$scope', 'adminService', '$location', '$window', '$http',
        function($scope, adminService, $location, $window, $http) {

            $scope.error = "";
            $scope.message = "";

            $scope.me = function() {

                adminService.me().then(
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

                adminService.generateSCIONLabVM(user).then(
                    function(data) {
                        console.log(data);
                        window.location.assign('/api/as/downloads?filename=' + data["filename"]);
                        $scope.message = data["message"];
                    },
                    function(response) {
                        console.log(response);
                        $scope.error = response.data;
                    });
            };

            $scope.removeSCIONLabVM = function(user) {
                $scope.error = "";
                $scope.message = "";

                adminService.removeSCIONLabVM(user).then(
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
