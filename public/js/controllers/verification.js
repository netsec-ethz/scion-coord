scionApp
    .controller('verificationCtrl', ['$scope', '$routeParams', '$location','verificationService', 
        function ($scope, $routeParams, $location, verificationService) {

            verificationService.verifyEmail($routeParams.uuid).then(
                function (response){ 
                    $scope.firstName = response.data.LastName;
                    $scope.lastName = response.data.FirstName;
                    $scope.activated = response.data.Activated;
                },
                function (response){
                    $location.path('/login');
                });
        }
    ]);
