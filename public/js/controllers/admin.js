angular.module('scionApp')
    .controller('adminCtrl', ['$scope', 'adminService', '$location', '$window', '$http',
        function($scope, adminService, $location, $window, $http) {

            $scope.me = function() {

                adminService.me().then(
                    function(data) {
                        console.log(data);
                        $scope.user = data;
                    },
                    function(response) {
                        console.log("RESPONSE is:");
                        console.log(response);
                        //$location.path('/');
                    });
            };


            // refresh the data when the controller is loaded
            $scope.me();

            $scope.scionLabVM = function(user) {
                $scope.error = "";

                adminService.scionLabVM(user).then(
                    function(data) {
                        console.log("DATA is:");
                        console.log(data);
                        window.location.assign('/api/as/downloads?filename=' + data);
                    },
                    function(response) {
                        console.log("RESPONSE is:");
                        console.log(response);
                        $scope.error = response.data;
                    });
            };
     }
    ]);
